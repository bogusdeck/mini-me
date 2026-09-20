package embed_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mini-me/internal/embed"
)

func generate768Vec(val float32) []float32 {
	vec := make([]float32, 768)
	for i := range vec {
		vec[i] = val
	}
	return vec
}

func TestNormalizeVector(t *testing.T) {
	raw := generate768Vec(2.0)
	normVec, err := embed.NormalizeVector(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(normVec) != 256 {
		t.Fatalf("expected 256 dimensions, got %d", len(normVec))
	}

	var sumSq float64
	for _, val := range normVec {
		sumSq += float64(val) * float64(val)
	}
	l2 := math.Sqrt(sumSq)

	if math.Abs(l2-1.0) > 1e-4 {
		t.Errorf("expected L2 norm 1.0, got %f", l2)
	}
}

func TestEmbedDocumentChunksAndQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req embed.EmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed decoding request: %v", err)
		}

		embeddings := make([][]float32, len(req.Input))
		for i, text := range req.Input {
			if !strings.HasPrefix(text, embed.DocPrefix) && !strings.HasPrefix(text, embed.QueryPrefix) {
				t.Errorf("expected text to have doc or query prefix, got %q", text)
			}
			embeddings[i] = generate768Vec(float32(i + 1))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(embed.EmbedResponse{
			Model:      req.Model,
			Embeddings: embeddings,
		})
	}))
	defer server.Close()

	client := embed.NewClient(embed.WithBaseURL(server.URL))
	ctx := context.Background()

	// Test Document Chunks
	docs := []string{"chunk 1", "chunk 2"}
	vecs, err := client.EmbedDocumentChunks(ctx, docs)
	if err != nil {
		t.Fatalf("failed embedding docs: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
	if len(vecs[0]) != 256 {
		t.Errorf("expected 256 dims, got %d", len(vecs[0]))
	}

	// Test Query
	queryVec, err := client.EmbedQuery(ctx, "what is mini-me?")
	if err != nil {
		t.Fatalf("failed embedding query: %v", err)
	}
	if len(queryVec) != 256 {
		t.Errorf("expected 256 dims, got %d", len(queryVec))
	}
}

func TestBatchingOver64(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var req embed.EmbedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		embeddings := make([][]float32, len(req.Input))
		for i := range req.Input {
			embeddings[i] = generate768Vec(1.0)
		}
		_ = json.NewEncoder(w).Encode(embed.EmbedResponse{Embeddings: embeddings})
	}))
	defer server.Close()

	client := embed.NewClient(embed.WithBaseURL(server.URL))
	ctx := context.Background()

	// 70 items should trigger 2 batch requests (64 + 6)
	items := make([]string, 70)
	for i := 0; i < 70; i++ {
		items[i] = "item"
	}

	vecs, err := client.EmbedDocumentChunks(ctx, items)
	if err != nil {
		t.Fatalf("failed batch embed: %v", err)
	}
	if len(vecs) != 70 {
		t.Fatalf("expected 70 vectors, got %d", len(vecs))
	}
	if callCount != 2 {
		t.Errorf("expected 2 batch HTTP calls, got %d", callCount)
	}
}

func TestCheckReachabilityAndModelPulled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Ollama is running"))
			return
		}
		if r.URL.Path == "/api/tags" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(embed.TagsResponse{
				Models: []embed.ModelInfo{
					{Name: "nomic-embed-text:latest", Model: "nomic-embed-text:latest"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := embed.NewClient(embed.WithBaseURL(server.URL))
	ctx := context.Background()

	if err := client.CheckReachability(ctx); err != nil {
		t.Fatalf("expected reachable, got err: %v", err)
	}

	pulled, err := client.CheckModelPulled(ctx)
	if err != nil {
		t.Fatalf("expected check model pulled to succeed: %v", err)
	}
	if !pulled {
		t.Errorf("expected model nomic-embed-text to be reported as pulled")
	}
}
