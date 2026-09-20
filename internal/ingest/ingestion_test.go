package ingest_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mini-me/internal/ingest"
	"mini-me/internal/store"
)

type mockEmbedder struct{}

func (m *mockEmbedder) EmbedDocumentChunks(ctx context.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i := range texts {
		vec := make([]float32, 256)
		vec[0] = 0.1 * float32(i+1)
		vecs[i] = vec
	}
	return vecs, nil
}

func (m *mockEmbedder) ModelName() string {
	return "nomic-embed-text"
}

func TestIngestAndDiffFile(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_ingest.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed opening store: %v", err)
	}
	defer st.Close()

	docPath := filepath.Join(tmpDir, "doc.md")
	initialContent := "# Section 1\nInitial content for doc.md."
	if err := os.WriteFile(docPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed writing doc.md: %v", err)
	}

	ingester := ingest.NewIngester(st, &mockEmbedder{})

	// 1. Initial Ingest
	stats, err := ingester.IngestPath(ctx, docPath, "test-proj")
	if err != nil {
		t.Fatalf("initial ingest failed: %v", err)
	}
	if stats.ChunksCreated != 1 {
		t.Errorf("expected 1 chunk created, got %d", stats.ChunksCreated)
	}

	chunks, err := st.GetChunksByPath(ctx, docPath)
	if err != nil || len(chunks) != 1 {
		t.Fatalf("expected 1 stored chunk, got %d (err: %v)", len(chunks), err)
	}

	// 2. Modify File (Change Section 1, add Section 2)
	updatedContent := "# Section 1\nModified content for doc.md.\n\n# Section 2\nNew section content."
	if err := os.WriteFile(docPath, []byte(updatedContent), 0644); err != nil {
		t.Fatalf("failed updating doc.md: %v", err)
	}

	stats2, err := ingester.IngestPath(ctx, docPath, "test-proj")
	if err != nil {
		t.Fatalf("second ingest failed: %v", err)
	}
	if stats2.ChunksCreated != 2 || stats2.ChunksDeleted != 1 {
		t.Errorf("expected 2 created, 1 deleted; got created=%d, deleted=%d", stats2.ChunksCreated, stats2.ChunksDeleted)
	}

	chunksAfter, err := st.GetChunksByPath(ctx, docPath)
	if err != nil || len(chunksAfter) != 2 {
		t.Fatalf("expected 2 stored chunks after update, got %d", len(chunksAfter))
	}

	// 3. Delete File & Reindex
	_ = os.Remove(docPath)
	reindexStats, err := ingester.Reindex(ctx)
	if err != nil {
		t.Fatalf("reindex failed: %v", err)
	}
	_ = reindexStats

	chunksFinal, err := st.GetChunksByPath(ctx, docPath)
	if err != nil || len(chunksFinal) != 0 {
		t.Errorf("expected 0 chunks after file deletion and reindex, got %d", len(chunksFinal))
	}
}
