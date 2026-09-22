package embed

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPurgeCache verifies that PurgeCache deletes the downloaded engine, model,
// archives and partial downloads, frees their bytes, and leaves the cache
// directory and unrelated files intact.
func TestPurgeCache(t *testing.T) {
	dir := t.TempDir()

	wiped := map[string][]byte{
		filepath.Join("models", "embeddinggemma.gguf"):   make([]byte, 300),
		filepath.Join("llama", "b10659", "llama-server"): make([]byte, 40),
		filepath.Join("llama", "b10659", "libllama.so"):  make([]byte, 20),
		"engine.tar.gz":   make([]byte, 50),
		"model.zip":       make([]byte, 10),
		"model.gguf.part": make([]byte, 5),
	}
	kept := map[string][]byte{
		"server.json":                          []byte(`{"url":"http://127.0.0.1:1","pid":1,"tag":"b10659","model":"m","started":"x"}`),
		filepath.Join("keep", "user-note.txt"): []byte("precious"),
	}
	var want int64
	for _, group := range []map[string][]byte{wiped, kept} {
		for name, body := range group {
			path := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, 0o644); err != nil {
				t.Fatal(err)
			}
			want += int64(len(body))
		}
	}

	freed, err := PurgeCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	expect := want - int64(len(kept["server.json"])) - int64(len(kept[filepath.Join("keep", "user-note.txt")]))
	if freed != expect {
		t.Fatalf("freed = %d, want %d", freed, expect)
	}

	for _, removed := range []string{
		filepath.Join(dir, "models"),
		filepath.Join(dir, "llama"),
		filepath.Join(dir, "engine.tar.gz"),
		filepath.Join(dir, "model.zip"),
		filepath.Join(dir, "model.gguf.part"),
		filepath.Join(dir, "server.json"),
	} {
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after purge", removed)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "keep", "user-note.txt")); err != nil {
		t.Fatalf("unrelated file removed: %v", err)
	}
}
