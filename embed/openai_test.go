package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestOpenAIEmbedOrderingAndBatch(t *testing.T) {
	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		var req openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		resp := openAIResponse{}
		for i := range req.Input {
			idx := len(req.Input) - 1 - i // reversed on purpose
			resp.Data = append(resp.Data, openAIEmbedding{Embedding: []float32{float32(idx)}, Index: idx})
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewOpenAI(srv.URL, "test")
	client.Batch = 2
	vectors, err := client.Embed(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 3 {
		t.Fatalf("vectors = %d, want 3", len(vectors))
	}
	if vectors[0][0] != 0 || vectors[1][0] != 1 || vectors[2][0] != 0 {
		t.Fatalf("order not preserved: %v", vectors)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestOpenAIRetry(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(openAIResponse{Data: []openAIEmbedding{{Embedding: []float32{1, 2}, Index: 0}}})
	}))
	defer srv.Close()

	client := NewOpenAI(srv.URL, "test")
	vectors, err := client.Embed(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 1 || vectors[0][0] != 1 {
		t.Fatalf("vectors = %v", vectors)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}
