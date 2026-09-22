package embed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// OpenAI talks to any OpenAI-compatible /v1/embeddings endpoint (llama-server,
// vLLM, Ollama's OpenAI API, hosted providers).
type OpenAI struct {
	BaseURL string
	Name    string
	Batch   int
	client  *http.Client
}

func NewOpenAI(baseURL, model string) *OpenAI {
	return &OpenAI{
		BaseURL: baseURL,
		Name:    model,
		Batch:   Batch(),
		client:  defaultHTTPClient(),
	}
}

func (o *OpenAI) Model() string { return o.Name }

type openAIRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbedding struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type openAIResponse struct {
	Data  []openAIEmbedding `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (o *OpenAI) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return batchVectors(o.Batch, texts, func(b []string) ([][]float32, error) {
		return o.embedBatch(ctx, b)
	})
}

func (o *OpenAI) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(openAIRequest{Model: o.Name, Input: texts})
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		vectors, retry, err := o.post(ctx, body, len(texts))
		if err == nil {
			return vectors, nil
		}
		lastErr = err
		if !retry {
			break
		}
	}
	return nil, fmt.Errorf("embed %s: %w", o.Name, lastErr)
}

func (o *OpenAI) post(ctx context.Context, body []byte, want int) ([][]float32, bool, error) {
	data, retry, err := postJSON(ctx, o.client, o.BaseURL+"/v1/embeddings", body)
	if err != nil {
		return nil, retry, err
	}
	var parsed openAIResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, false, err
	}
	if parsed.Error != nil {
		return nil, false, fmt.Errorf("embeddings: %s", parsed.Error.Message)
	}
	if len(parsed.Data) != want {
		return nil, false, fmt.Errorf("embeddings returned %d vectors for %d texts", len(parsed.Data), want)
	}
	sort.Slice(parsed.Data, func(i, j int) bool { return parsed.Data[i].Index < parsed.Data[j].Index })
	vectors := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		vectors[i] = d.Embedding
	}
	return vectors, false, nil
}
