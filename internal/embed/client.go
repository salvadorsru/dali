package embed

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// batchVectors chunks texts by the batch size and concatenates the provider's
// vectors, preserving input order.
func batchVectors(batch int, texts []string, embed func([]string) ([][]float32, error)) ([][]float32, error) {
	if batch <= 0 {
		batch = defaultBatch
	}
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += batch {
		end := min(start+batch, len(texts))
		vectors, err := embed(texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, vectors...)
	}
	return out, nil
}

// postJSON posts a JSON body and returns the raw response. Transient errors
// (network, 5xx) are flagged for retry; 4xx and below are not.
func postJSON(ctx context.Context, client *http.Client, url string, body []byte) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	if resp.StatusCode == http.StatusOK {
		return data, false, nil
	}
	return nil, resp.StatusCode >= 500, fmt.Errorf("embed status %d: %s", resp.StatusCode, data)
}