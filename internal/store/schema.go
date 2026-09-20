package store

import (
	"context"
	"database/sql"
	"fmt"

	"mini-me/internal/logger"
)

type Migration struct {
	Version int
	SQL     string
}

var migrations = []Migration{
	{
		Version: 1,
		SQL: `
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chunk (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL,
    path TEXT NOT NULL,
    project TEXT NOT NULL DEFAULT '',
    text TEXT NOT NULL,
    hash TEXT NOT NULL,
    ts INTEGER NOT NULL,
    embed_model TEXT NOT NULL,
    chunker_version TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_chunk_path ON chunk(path);
CREATE INDEX IF NOT EXISTS idx_chunk_project ON chunk(project);
CREATE INDEX IF NOT EXISTS idx_chunk_hash ON chunk(hash);

CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vec USING vec0(
    embedding float[256]
);

CREATE VIRTUAL TABLE IF NOT EXISTS chunk_fts USING fts5(
    text,
    content='chunk',
    content_rowid='id'
);

CREATE TABLE IF NOT EXISTS entity (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (type IN ('person', 'org', 'project', 'topic', 'place')),
    name TEXT NOT NULL UNIQUE,
    aliases TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS fact (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    subject_id INTEGER NOT NULL REFERENCES entity(id) ON DELETE CASCADE,
    predicate TEXT NOT NULL,
    object TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('proposed', 'confirmed', 'rejected')),
    confidence REAL NOT NULL DEFAULT 1.0,
    sensitivity TEXT NOT NULL CHECK (sensitivity IN ('normal', 'personal', 'never_infer')),
    valid_from TIMESTAMP,
    valid_to TIMESTAMP,
    superseded_by INTEGER REFERENCES fact(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_fact_subject ON fact(subject_id);
CREATE INDEX IF NOT EXISTS idx_fact_status ON fact(status);

CREATE TABLE IF NOT EXISTS fact_source (
    fact_id INTEGER NOT NULL REFERENCES fact(id) ON DELETE CASCADE,
    chunk_id INTEGER NOT NULL REFERENCES chunk(id) ON DELETE CASCADE,
    PRIMARY KEY (fact_id, chunk_id)
);

CREATE TABLE IF NOT EXISTS field (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`,
	},
}

// Migrate applies pending migrations idempotently.
func Migrate(ctx context.Context, db *sql.DB) error {
	// Create schema_version table if missing first
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to ensure schema_version table: %w", err)
	}

	for _, m := range migrations {
		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_version WHERE version = ?", m.Version).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check migration version %d: %w", m.Version, err)
		}

		if count == 0 {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to start migration tx for version %d: %w", m.Version, err)
			}

			if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("failed executing migration version %d: %w", m.Version, err)
			}

			if _, err := tx.ExecContext(ctx, "INSERT INTO schema_version (version) VALUES (?)", m.Version); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("failed logging migration version %d: %w", m.Version, err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed committing migration version %d: %w", m.Version, err)
			}
			logger.Info("applied database migration", "version", m.Version)
		}
	}
	return nil
}
