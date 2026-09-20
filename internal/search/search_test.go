package search_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mini-me/internal/search"
	"mini-me/internal/store"
)

type mockEmbedder struct{}

func (m *mockEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	vec := make([]float32, 256)
	vec[0] = 0.5
	return vec, nil
}

func TestHybridSearchAndStaleness(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_search.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed opening store: %v", err)
	}
	defer st.Close()

	// 1. Create a valid file on disk
	validFilePath := filepath.Join(tmpDir, "valid.md")
	validContent := "Breadcrumb: # Architecture > ## Database\n\nSQLite database in WAL mode."
	if err := os.WriteFile(validFilePath, []byte(validContent), 0644); err != nil {
		t.Fatalf("failed creating valid file: %v", err)
	}

	validChunk := &store.Chunk{
		SourceType:     "markdown",
		Path:           validFilePath,
		Project:        "mini-me",
		Text:           validContent,
		Hash:           "validhash123",
		TS:             time.Now().Unix(),
		EmbedModel:     "nomic-embed-text:256",
		ChunkerVersion: "v1",
	}

	vec1 := make([]float32, 256)
	vec1[0] = 0.5
	id1, err := st.InsertChunk(ctx, validChunk, vec1)
	if err != nil {
		t.Fatalf("failed inserting valid chunk: %v", err)
	}

	// 2. Create a chunk for a DELETED file
	staleFilePath := filepath.Join(tmpDir, "deleted.md")
	staleChunk := &store.Chunk{
		SourceType:     "markdown",
		Path:           staleFilePath,
		Project:        "mini-me",
		Text:           "This file was deleted on disk.",
		Hash:           "stalehash123",
		TS:             time.Now().Unix(),
		EmbedModel:     "nomic-embed-text:256",
		ChunkerVersion: "v1",
	}

	vec2 := make([]float32, 256)
	vec2[0] = 0.5
	id2, err := st.InsertChunk(ctx, staleChunk, vec2)
	if err != nil {
		t.Fatalf("failed inserting stale chunk: %v", err)
	}

	// 3. Perform Hybrid Search for "SQLite"
	engine := search.NewSearchEngine(st, &mockEmbedder{})
	hits, err := engine.Search(ctx, "SQLite", search.SearchOptions{K: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	// Stale hit (id2) should have been dropped and purged, valid hit (id1) returned
	if len(hits) != 1 {
		t.Fatalf("expected 1 valid search hit, got %d", len(hits))
	}

	if hits[0].Chunk.ID != id1 {
		t.Errorf("expected hit chunk id %d, got %d", id1, hits[0].Chunk.ID)
	}

	// Verify stale chunk was purged from store
	purgedChunk, err := st.GetChunk(ctx, id2)
	if err != nil {
		t.Fatalf("err checking purged chunk: %v", err)
	}
	if purgedChunk != nil {
		t.Errorf("expected stale chunk to be purged from database")
	}
}
