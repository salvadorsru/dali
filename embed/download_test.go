package embed

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssetSuffix(t *testing.T) {
	cases := []struct{ goos, goarch, want string }{
		{"darwin", "arm64", "bin-macos-arm64"},
		{"darwin", "amd64", "bin-macos-x64"},
		{"linux", "amd64", "bin-ubuntu-x64"},
		{"linux", "arm64", "bin-ubuntu-arm64"},
		{"windows", "amd64", "bin-win-cpu-x64"},
	}
	for _, c := range cases {
		got, err := assetSuffix(c.goos, c.goarch)
		if err != nil || got != c.want {
			t.Fatalf("assetSuffix(%s,%s) = %q, %v", c.goos, c.goarch, got, err)
		}
	}
	if _, err := assetSuffix("plan9", "amd64"); err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}

func TestSafeJoin(t *testing.T) {
	dir := t.TempDir()
	got, err := safeJoin(dir, "../escape")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, filepath.Clean(dir)+string(os.PathSeparator)) {
		t.Fatalf("traversal escaped destination: %q", got)
	}
	got, err = safeJoin(dir, "bin/llama-server")
	if err != nil || got != filepath.Join(dir, "bin/llama-server") {
		t.Fatalf("safeJoin = %q, %v", got, err)
	}
}

func TestExtractTarGz(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\n")
	tw.WriteHeader(&tar.Header{Name: "llama-server", Mode: 0o755, Size: int64(len(body))})
	tw.Write(body)
	tw.WriteHeader(&tar.Header{Name: "libllama.so.0.1.0", Mode: 0o644, Size: 2})
	tw.Write([]byte("so"))
	tw.WriteHeader(&tar.Header{Name: "libllama.so.0", Typeflag: tar.TypeSymlink, Linkname: "libllama.so.0.1.0", Mode: 0o777})
	tw.Close()
	gz.Close()

	archive := filepath.Join(t.TempDir(), "llama.tar.gz")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := extractArchive(archive, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "llama-server"))
	if err != nil || string(got) != string(body) {
		t.Fatalf("extracted = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "libllama.so.0")); err != nil {
		t.Fatalf("symlink missing: %v", err)
	}
}

func TestExtractZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("llama-server.exe")
	w.Write([]byte("MZ"))
	zw.Close()

	archive := filepath.Join(t.TempDir(), "llama.zip")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := extractArchive(archive, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "llama-server.exe")); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadChecksum(t *testing.T) {
	body := []byte("model-bytes")
	sum := sha256.Sum256(body)
	want := hex.EncodeToString(sum[:])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "model.gguf")
	if err := download(context.Background(), srv.URL, dest, want); err != nil {
		t.Fatal(err)
	}
	if err := download(context.Background(), srv.URL, dest+".bad", "deadbeef"); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestShaFromDigest(t *testing.T) {
	if got := shaFromDigest("sha256:abc"); got != "abc" {
		t.Fatalf("shaFromDigest = %q", got)
	}
}

func TestHFSiblingSha(t *testing.T) {
	s := hfSibling{}
	s.LFS = &struct {
		OID    string `json:"oid"`
		SHA256 string `json:"sha256"`
		Size   int64  `json:"size"`
	}{SHA256: "abc"}
	if s.sha() != "abc" {
		t.Fatalf("sha = %q", s.sha())
	}
}

func TestResolveLive(t *testing.T) {
	if os.Getenv("DALI_NET_TEST") == "" {
		t.Skip("set DALI_NET_TEST=1 to hit GitHub/HuggingFace")
	}
	asset, err := resolveAsset(context.Background(), DefaultLlamaTag, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if asset.Digest == "" {
		t.Fatal("release asset has no digest")
	}
	sibling, err := resolveModel(context.Background(), DefaultModelRepo, "")
	if err != nil {
		t.Fatal(err)
	}
	if sibling.sha() == "" {
		t.Fatalf("model %s has no checksum", sibling.Filename)
	}
}
