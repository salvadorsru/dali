package embed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Ollama struct {
	URL    string
	Name   string
	Batch  int
	client *http.Client
}

func defaultHTTPClient() *http.Client { return &http.Client{Timeout: 5 * time.Minute} }

func (o *Ollama) Model() string { return o.Name }

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Error      string      `json:"error"`
}

func (o *Ollama) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return batchVectors(o.Batch, texts, func(b []string) ([][]float32, error) {
		return o.embedBatch(ctx, b)
	})
}

func (o *Ollama) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(embedRequest{Model: o.Name, Input: texts})
	if err != nil {
		return nil, err
	}
	return retry(ctx, o.Name, func() ([][]float32, bool, error) {
		return o.post(ctx, body)
	})
}

func (o *Ollama) post(ctx context.Context, body []byte) ([][]float32, bool, error) {
	data, retry, err := postJSON(ctx, o.client, o.URL+"/api/embed", body)
	if err != nil {
		return nil, retry, err
	}
	var parsed embedResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, false, err
	}
	if parsed.Error != "" {
		return nil, false, fmt.Errorf("ollama: %s", parsed.Error)
	}
	if len(parsed.Embeddings) != 0 {
		return parsed.Embeddings, false, nil
	}
	return nil, false, fmt.Errorf("ollama returned no embeddings")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
