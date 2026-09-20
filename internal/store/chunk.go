package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	ncrucesvec "github.com/asg017/sqlite-vec-go-bindings/ncruces"
)

type Chunk struct {
	ID             int64     `json:"id"`
	SourceType     string    `json:"source_type"`
	Path           string    `json:"path"`
	Project        string    `json:"project"`
	Text           string    `json:"text"`
	Hash           string    `json:"hash"`
	TS             int64     `json:"ts"`
	EmbedModel     string    `json:"embed_model"`
	ChunkerVersion string    `json:"chunker_version"`
}

// InsertChunk inserts a chunk row, its corresponding vector in chunk_vec, and its text in chunk_fts in a single transaction.
func (s *Store) InsertChunk(ctx context.Context, chunk *Chunk, embedding []float32) (int64, error) {
	if chunk.TS == 0 {
		chunk.TS = time.Now().Unix()
	}

	vecBytes, err := ncrucesvec.SerializeFloat32(embedding)
	if err != nil {
		return 0, fmt.Errorf("failed to serialize float32 embedding: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO chunk (source_type, path, project, text, hash, ts, embed_model, chunker_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, chunk.SourceType, chunk.Path, chunk.Project, chunk.Text, chunk.Hash, chunk.TS, chunk.EmbedModel, chunk.ChunkerVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to insert chunk: %w", err)
	}

	chunkID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}
	chunk.ID = chunkID

	if len(embedding) > 0 {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO chunk_vec(rowid, embedding) VALUES (?, ?)
		`, chunkID, vecBytes)
		if err != nil {
			return 0, fmt.Errorf("failed to insert chunk_vec: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO chunk_fts(rowid, text) VALUES (?, ?)
	`, chunkID, chunk.Text)
	if err != nil {
		return 0, fmt.Errorf("failed to insert chunk_fts: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit insert chunk tx: %w", err)
	}

	return chunkID, nil
}

// DeleteChunk removes a chunk and keeps chunk_vec and chunk_fts in sync within a transaction.
func (s *Store) DeleteChunk(ctx context.Context, chunkID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var text string
	err = tx.QueryRowContext(ctx, "SELECT text FROM chunk WHERE id = ?", chunkID).Scan(&text)
	if err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to fetch chunk for deletion: %w", err)
	}

	// Delete from chunk_vec
	if _, err := tx.ExecContext(ctx, "DELETE FROM chunk_vec WHERE rowid = ?", chunkID); err != nil {
		return fmt.Errorf("failed to delete from chunk_vec: %w", err)
	}

	// Delete from chunk_fts
	if _, err := tx.ExecContext(ctx, "INSERT INTO chunk_fts(chunk_fts, rowid, text) VALUES('delete', ?, ?)", chunkID, text); err != nil {
		return fmt.Errorf("failed to delete from chunk_fts: %w", err)
	}

	// Delete from chunk
	if _, err := tx.ExecContext(ctx, "DELETE FROM chunk WHERE id = ?", chunkID); err != nil {
		return fmt.Errorf("failed to delete chunk row: %w", err)
	}

	return tx.Commit()
}

// DeleteChunksByPath removes all chunks associated with a file path, keeping vec and fts in sync.
func (s *Store) DeleteChunksByPath(ctx context.Context, path string) error {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM chunk WHERE path = ?", path)
	if err != nil {
		return fmt.Errorf("failed to query chunks by path: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed scanning chunk id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range ids {
		if err := s.DeleteChunk(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// GetChunk retrieves a chunk by ID.
func (s *Store) GetChunk(ctx context.Context, id int64) (*Chunk, error) {
	c := &Chunk{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, source_type, path, project, text, hash, ts, embed_model, chunker_version
		FROM chunk WHERE id = ?
	`, id).Scan(&c.ID, &c.SourceType, &c.Path, &c.Project, &c.Text, &c.Hash, &c.TS, &c.EmbedModel, &c.ChunkerVersion)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}
	return c, nil
}

// GetChunksByPath retrieves all chunks associated with a given path.
func (s *Store) GetChunksByPath(ctx context.Context, path string) ([]*Chunk, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, source_type, path, project, text, hash, ts, embed_model, chunker_version
		FROM chunk WHERE path = ? ORDER BY id ASC
	`, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get chunks by path: %w", err)
	}
	defer rows.Close()

	var chunks []*Chunk
	for rows.Next() {
		c := &Chunk{}
		if err := rows.Scan(&c.ID, &c.SourceType, &c.Path, &c.Project, &c.Text, &c.Hash, &c.TS, &c.EmbedModel, &c.ChunkerVersion); err != nil {
			return nil, fmt.Errorf("failed scanning chunk row: %w", err)
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}
