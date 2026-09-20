package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mini-me/internal/store"
)

func TestOpenAndMigrationIdempotency(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_store.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}

	// Verify file mode 0600
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("failed to stat db file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected file mode 0600, got %o", info.Mode().Perm())
	}

	// Test migration idempotency by running Migrate again
	if err := store.Migrate(ctx, st.DB()); err != nil {
		t.Fatalf("second migration call failed: %v", err)
	}

	var count int
	if err := st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_version").Scan(&count); err != nil {
		t.Fatalf("failed to query schema_version: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 migration row, got %d", count)
	}

	_ = st.Close()
}

func TestChunkVecFtsSync(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_chunk_sync.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer st.Close()

	// 1. Insert chunk with 256d dummy vector
	dummyVec := make([]float32, 256)
	dummyVec[0] = 0.5
	dummyVec[255] = -0.5

	chunk := &store.Chunk{
		SourceType:     "markdown",
		Path:           "/path/to/doc.md",
		Project:        "mini-me",
		Text:           "This is a test note about Antigravity AI coding assistant.",
		Hash:           "hash123",
		EmbedModel:     "nomic-embed-text:256",
		ChunkerVersion: "v1",
	}

	id, err := st.InsertChunk(ctx, chunk, dummyVec)
	if err != nil {
		t.Fatalf("failed inserting chunk: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive chunk id, got %d", id)
	}

	// Verify chunk table row
	retrievedChunk, err := st.GetChunk(ctx, id)
	if err != nil || retrievedChunk == nil {
		t.Fatalf("failed retrieving chunk %d: %v", id, err)
	}
	if retrievedChunk.Text != chunk.Text {
		t.Errorf("expected text %q, got %q", chunk.Text, retrievedChunk.Text)
	}

	// Verify vec0 table sync
	var vecCount int
	if err := st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM chunk_vec WHERE rowid = ?", id).Scan(&vecCount); err != nil {
		t.Fatalf("failed checking chunk_vec: %v", err)
	}
	if vecCount != 1 {
		t.Errorf("expected 1 row in chunk_vec for rowid %d, got %d", id, vecCount)
	}

	// Verify FTS5 table sync
	var ftsCount int
	if err := st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM chunk_fts WHERE chunk_fts MATCH 'Antigravity'").Scan(&ftsCount); err != nil {
		t.Fatalf("failed checking chunk_fts match: %v", err)
	}
	if ftsCount != 1 {
		t.Errorf("expected 1 row matching FTS query, got %d", ftsCount)
	}

	// 2. Delete chunk and verify deletion sync across all tables
	if err := st.DeleteChunk(ctx, id); err != nil {
		t.Fatalf("failed deleting chunk: %v", err)
	}

	// Verify main chunk row is gone
	deletedChunk, err := st.GetChunk(ctx, id)
	if err != nil {
		t.Fatalf("error checking deleted chunk: %v", err)
	}
	if deletedChunk != nil {
		t.Errorf("expected chunk to be nil after deletion")
	}

	// Verify chunk_vec is empty
	if err := st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM chunk_vec WHERE rowid = ?", id).Scan(&vecCount); err != nil {
		t.Fatalf("failed checking chunk_vec post deletion: %v", err)
	}
	if vecCount != 0 {
		t.Errorf("expected 0 rows in chunk_vec post deletion, got %d", vecCount)
	}

	// Verify chunk_fts is updated
	if err := st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM chunk_fts WHERE chunk_fts MATCH 'Antigravity'").Scan(&ftsCount); err != nil {
		t.Fatalf("failed checking chunk_fts post deletion: %v", err)
	}
	if ftsCount != 0 {
		t.Errorf("expected 0 rows matching FTS post deletion, got %d", ftsCount)
	}
}

func TestEntityFactFieldOperations(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_graph.db")

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed opening store: %v", err)
	}
	defer st.Close()

	// Entity
	ent := &store.Entity{
		Type: store.EntityPerson,
		Name: "Alice",
	}
	entID, err := st.CreateEntity(ctx, ent)
	if err != nil {
		t.Fatalf("failed creating entity: %v", err)
	}

	// Fact
	fact := &store.Fact{
		SubjectID:   entID,
		Predicate:   "role",
		Object:      "Senior Engineer",
		Status:      store.FactStatusConfirmed,
		Sensitivity: store.SensitivityNormal,
	}
	factID, err := st.CreateFact(ctx, fact)
	if err != nil {
		t.Fatalf("failed creating fact: %v", err)
	}
	if factID <= 0 {
		t.Errorf("expected positive fact id, got %d", factID)
	}

	// Field
	if err := st.SetField(ctx, "full_name", "Alice Bob"); err != nil {
		t.Fatalf("failed setting field: %v", err)
	}
	field, err := st.GetField(ctx, "full_name")
	if err != nil || field == nil {
		t.Fatalf("failed getting field: %v", err)
	}
	if field.Value != "Alice Bob" {
		t.Errorf("expected field value 'Alice Bob', got %q", field.Value)
	}
}
