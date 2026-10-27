package mcp

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/salvadorsru/dali/internal/index"
	"github.com/salvadorsru/dali/internal/query"
)

// bound is a closable service that never parses a design, so cache tests do not
// depend on a sample .fig.
func bound(t *testing.T) *query.Service {
	t.Helper()
	db, err := index.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	return query.Bind(db)
}

// TestOpenCache pins the bounded cache: it never holds more than capacity
// services, evicts and closes the least recently used one, and never evicts the
// active project even when it is the oldest entry.
func TestOpenCache(t *testing.T) {
	_, h := newServer("", t.TempDir(), nil)
	defer h.close()
	h.active = "active"

	services := map[string]*query.Service{}
	store := func(path string) {
		svc := bound(t)
		services[path] = svc
		h.store(path, svc)
	}

	for _, path := range []string{"active", "a", "b", "c"} {
		store(path)
	}
	for _, path := range []string{"d", "e", "f", "g"} {
		store(path)
		if len(h.opened) > capacity {
			t.Fatalf("cache holds %d services, capacity %d", len(h.opened), capacity)
		}
		if _, ok := h.opened["active"]; !ok {
			t.Fatalf("active project evicted after storing %s", path)
		}
	}

	if len(h.opened) != capacity {
		t.Fatalf("cache holds %d services, want %d", len(h.opened), capacity)
	}
	if len(h.used) != capacity || h.used[0] != "active" {
		t.Fatalf("access order = %v, want active first", h.used)
	}
	if err := services["a"].DB().SQL().Ping(); err == nil {
		t.Fatal("evicted service was not closed")
	}
}

// TestOpenParallel: the open lock is per path, so an in-flight open of one
// project never serialises another, and two fresh projects open concurrently.
func TestOpenParallel(t *testing.T) {
	root := t.TempDir()
	paths := map[string]string{}
	for _, name := range []string{"a", "b", "c", "d"} {
		paths[name] = emptyFig(t, filepath.Join(root, name+".fig"))
	}
	_, h := newServer("", root, nil)
	defer h.close()
	note := &notifier{out: io.Discard}

	lock := h.lock(paths["a"])
	lock.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := h.service("b", note)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("opening b while a is opening: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("opening b blocked by an in-flight open of a")
	}
	lock.Unlock()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{"c", "d"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := h.service(id, note)
			errs <- err
		}(id)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("parallel open: %v", err)
		}
	}
	for _, id := range []string{"b", "c", "d"} {
		if _, ok := h.opened[paths[id]]; !ok {
			t.Fatalf("%s not cached after opening", id)
		}
	}
}

// emptyFig writes the smallest valid .fig: an empty Kiwi Message, enough for
// parser.Open + index.Write to succeed without committing a sample design.
func emptyFig(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var canvas bytes.Buffer
	canvas.WriteString("fig-kiwi")
	canvas.Write(make([]byte, 4))
	chunk(t, &canvas, []byte{1, 'M', 'e', 's', 's', 'a', 'g', 'e', 0, 2, 0})
	chunk(t, &canvas, []byte{0})

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.Create("canvas.fig")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(canvas.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, archive.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func chunk(t *testing.T, w io.Writer, raw []byte) {
	t.Helper()
	var compressed bytes.Buffer
	fw, err := flate.NewWriter(&compressed, flate.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(compressed.Len()))
	if _, err := w.Write(size[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(compressed.Bytes()); err != nil {
		t.Fatal(err)
	}
}
