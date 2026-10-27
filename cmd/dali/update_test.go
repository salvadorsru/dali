package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// release serves a fake GitHub release: latest tag, the archive for this
// platform and its checksums.
func release(t *testing.T, tag string, bin []byte, checksum bool) *httptest.Server {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "dali", Mode: 0o755, Size: int64(len(bin)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(bin); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	archive := buf.Bytes()

	sum := sha256.Sum256(archive)
	name := assetName()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			w.Write([]byte(`{"tag_name":"` + tag + `"}`))
		case "/releases/download/" + tag + "/" + name:
			w.Write(archive)
		case "/releases/download/" + tag + "/checksums.txt":
			if !checksum {
				w.Write([]byte("deadbeef  " + name + "\n"))
				return
			}
			w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	releaseBase, latestAPI = srv.URL+"/releases/download", srv.URL+"/latest"
	t.Cleanup(srv.Close)
	return srv
}

// TestUpdate pins the self-update cycle: resolve the latest tag, download the
// archive for this platform, verify it and swap the binary in place.
func TestUpdate(t *testing.T) {
	old := version
	version = "0.1.0"
	t.Cleanup(func() { version = old })

	bin := []byte("#!/bin/sh\necho dali 9.9.9\n")
	release(t, "v9.9.9", bin, true)

	exe := filepath.Join(t.TempDir(), "dali")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := update(exe); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(bin) {
		t.Fatalf("binary = %q, want %q", got, bin)
	}
	if info, err := os.Stat(exe); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, %v; want 0755", info.Mode(), err)
	}

	// A binary already at the latest release must not download anything.
	version = "9.9.9"
	if err := update(exe); err != nil {
		t.Fatalf("up to date: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != string(bin) {
		t.Fatalf("up-to-date run rewrote the binary: %q", got)
	}
}

// TestUpdateChecksum: a mismatch stops the replacement and leaves the running
// binary untouched.
func TestUpdateChecksum(t *testing.T) {
	old := version
	version = "0.1.0"
	t.Cleanup(func() { version = old })

	release(t, "v9.9.9", []byte("new"), false)

	exe := filepath.Join(t.TempDir(), "dali")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := update(exe); err == nil {
		t.Fatal("expected a checksum error")
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Fatalf("binary replaced despite a bad checksum: %q", got)
	}
}

func TestAssetName(t *testing.T) {
	want := "dali_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if runtime.GOOS == "windows" {
		want = "dali_" + runtime.GOOS + "_" + runtime.GOARCH + ".zip"
	}
	if got := assetName(); got != want {
		t.Fatalf("assetName = %q, want %q", got, want)
	}
}
