package embed

import "context"

// Embedder turns text into vectors. One implementation ships (Ollama); the
// interface keeps the rest of Dalí provider-agnostic.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Model() string
}
