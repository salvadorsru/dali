package embed

import "context"

// Embedder turns text into vectors. Implementations talk to an
// OpenAI-compatible embeddings endpoint or a managed llama-server; the
// interface keeps the rest of Dalí provider-agnostic.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Model() string
}
