package mcp

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/salvadorsru/dali/embed"
)

type fakeEmbedder struct{ calls atomic.Int32 }

func (f *fakeEmbedder) Model() string { return "fake" }

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	f.calls.Add(1)
	out := make([][]float32, len(texts))
	for i, t := range texts {
		h := sha256.Sum256([]byte(t))
		out[i] = []float32{float32(h[0]), float32(h[1]), float32(h[2]), float32(h[3])}
	}
	return out, nil
}

func (panicEmbedder) Model() string { return "panic" }

func (panicEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	panic("embedder must not be used by searches")
}

type panicEmbedder struct{}

func tmpFig(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(sample); err != nil {
		t.Skip("sample fig not present")
	}
	data, err := os.ReadFile(sample)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "woments.fig")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func handlerFor(t *testing.T, emb embed.Embedder) *handlers {
	t.Helper()
	_, h := newServer(tmpFig(t), emb)
	t.Cleanup(h.close)
	return h
}

func callTool(name string) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Meta: mcp.Meta{}, Name: name},
		Extra:  &mcp.RequestExtra{},
	}
}

// TestMCPEmbedderOptIn pins the embedder gate: without DALI_EMBEDDINGS=1 the
// MCP never constructs an embedder, and a provider of off/none also yields nil.
func TestMCPEmbedderOptIn(t *testing.T) {
	t.Setenv("DALI_EMBEDDINGS", "")
	t.Setenv("DALI_EMBED_PROVIDER", "")
	if emb := mcpEmbedder(); emb != nil {
		t.Fatalf("no opt-in => embedder = %v", emb)
	}
	t.Setenv("DALI_EMBED_PROVIDER", "off")
	if emb := embed.FromEnv(); emb != nil {
		t.Fatalf("provider off => embedder = %v", emb)
	}
	if emb := mcpEmbedder(); emb != nil {
		t.Fatalf("opt-in absent + provider off => embedder = %v", emb)
	}
	t.Setenv("DALI_EMBEDDINGS", "1")
	t.Setenv("DALI_EMBED_PROVIDER", "none")
	if emb := embed.FromEnv(); emb != nil {
		t.Fatalf("provider none => embedder = %v", emb)
	}
	t.Setenv("DALI_EMBED_PROVIDER", "")
	if emb := mcpEmbedder(); emb == nil {
		t.Fatal("opt-in with default provider => expected an embedder")
	}
}

// TestEmbeddingGate: without an embedder, semantic and index_embeddings fail
// fast and hybrid falls back to full-text, never erroring.
func TestEmbeddingGate(t *testing.T) {
	h := handlerFor(t, nil)
	ctx := context.Background()

	_, _, err := h.semanticSearch(ctx, callTool("semantic_search_figma"), semanticInput{Query: "payment"})
	if err == nil || !strings.Contains(err.Error(), "embeddings disabled") {
		t.Fatalf("semantic disabled = %v", err)
	}
	_, _, err = h.indexEmbeddings(ctx, callTool("index_embeddings"), fileInput{})
	if err == nil || !strings.Contains(err.Error(), "embeddings disabled") {
		t.Fatalf("index disabled = %v", err)
	}
	_, out, err := h.hybridSearch(ctx, callTool("hybrid_search_figma"), hybridInput{Query: "menopausia", Limit: 5})
	if err != nil {
		t.Fatalf("hybrid fallback = %v", err)
	}
	if out.UsedEmbeddings || len(out.Results) == 0 {
		t.Fatalf("hybrid fallback = %+v", out)
	}
}

// TestHybridNeverIndexes: even a configured embedder is never called by hybrid
// when the vector index does not exist; it panics if touched.
func TestHybridNeverIndexes(t *testing.T) {
	h := handlerFor(t, panicEmbedder{})
	_, out, err := h.hybridSearch(context.Background(), callTool("hybrid_search_figma"), hybridInput{Query: "menopausia", Limit: 5})
	if err != nil {
		t.Fatalf("hybrid = %v", err)
	}
	if out.UsedEmbeddings || len(out.Results) == 0 {
		t.Fatalf("hybrid without index = %+v", out)
	}
}

// TestEmbeddingsOptInFlow walks the full contract: no index -> FTS; index via
// index_embeddings only; semantic and hybrid work once vectors exist.
func TestEmbeddingsOptInFlow(t *testing.T) {
	emb := &fakeEmbedder{}
	h := handlerFor(t, emb)
	ctx := context.Background()

	_, _, err := h.semanticSearch(ctx, callTool("semantic_search_figma"), semanticInput{Query: "payment"})
	if err == nil || !strings.Contains(err.Error(), "vector index missing") {
		t.Fatalf("semantic before index = %v", err)
	}
	if emb.calls.Load() != 0 {
		t.Fatalf("semantic must not embed without an index (%d calls)", emb.calls.Load())
	}

	_, out, err := h.hybridSearch(ctx, callTool("hybrid_search_figma"), hybridInput{Query: "menopausia", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if out.UsedEmbeddings {
		t.Fatalf("hybrid before index used embeddings: %+v", out)
	}

	_, idx, err := h.indexEmbeddings(ctx, callTool("index_embeddings"), fileInput{})
	if err != nil {
		t.Fatalf("index = %v", err)
	}
	if idx.Indexed == 0 || emb.calls.Load() == 0 {
		t.Fatalf("index = %+v calls=%d", idx, emb.calls.Load())
	}

	if _, _, err := h.semanticSearch(ctx, callTool("semantic_search_figma"), semanticInput{Query: "menopausia", Limit: 5}); err != nil {
		t.Fatalf("semantic after index = %v", err)
	}

	_, out, err = h.hybridSearch(ctx, callTool("hybrid_search_figma"), hybridInput{Query: "menopausia", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !out.UsedEmbeddings || len(out.Results) == 0 {
		t.Fatalf("hybrid after index = %+v", out)
	}
}
