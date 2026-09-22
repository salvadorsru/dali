package embed

import (
	"context"
	"os"
	"testing"
)

// TestManagedModelIdentity guards against the identity being empty, which
// silently defeated model-change detection and reindexing.
func TestManagedModelIdentity(t *testing.T) {
	t.Setenv("DALI_LLAMA_MODEL", "")
	t.Setenv("DALI_LLAMA_FILE", "")

	if got := managedFromEnv().Model(); got != DefaultModelRepo {
		t.Fatalf("default identity = %q, want %q", got, DefaultModelRepo)
	}
	if got := (&Managed{}).Model(); got != DefaultModelRepo {
		t.Fatalf("zero-value identity = %q, want %q", got, DefaultModelRepo)
	}
	if got := (&Managed{ModelRepo: "org/repo", ModelFile: "m.gguf"}).Model(); got != "org/repo:m.gguf" {
		t.Fatalf("explicit identity = %q", got)
	}
}

// TestManagedLive downloads the official llama.cpp binary and GGUF model, then
// embeds two texts through the managed server. Network + ~350MB; opt-in.
func TestManagedLive(t *testing.T) {
	if os.Getenv("DALI_MANAGED_TEST") == "" {
		t.Skip("set DALI_MANAGED_TEST=1 to download llama.cpp + model")
	}
	m := &Managed{
		CacheDir: os.Getenv("DALI_CACHE_DIR"),
		Confirm:  func(string) bool { return true },
	}
	defer m.Close()

	vectors, err := m.Embed(context.Background(), []string{"hello world", "botón de compra"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 2 || len(vectors[0]) == 0 {
		t.Fatalf("vectors = %d x %d", len(vectors), len(vectors[0]))
	}
	if len(vectors[0]) != len(vectors[1]) {
		t.Fatalf("dimension mismatch: %d vs %d", len(vectors[0]), len(vectors[1]))
	}
	t.Logf("dimensions=%d", len(vectors[0]))
}
