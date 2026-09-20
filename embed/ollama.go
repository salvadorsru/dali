package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	batch := o.Batch
	if batch <= 0 {
		batch = defaultBatch
	}
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += batch {
		end := start + batch
		if end > len(texts) {
			end = len(texts)
		}
		vectors, err := o.embedBatch(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, vectors...)
	}
	return out, nil
}

func (o *Ollama) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(embedRequest{Model: o.Name, Input: texts})
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
		vectors, retry, err := o.post(ctx, body)
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

func (o *Ollama) post(ctx context.Context, body []byte) ([][]float32, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.URL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	if resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("ollama status %d: %s", resp.StatusCode, data)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("ollama status %d: %s", resp.StatusCode, data)
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
