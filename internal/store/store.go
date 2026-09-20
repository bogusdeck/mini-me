package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/asg017/sqlite-vec-go-bindings/ncruces"
	"github.com/ncruces/go-sqlite3"
	_ "github.com/ncruces/go-sqlite3/driver"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"

	"mini-me/internal/config"
)

func init() {
	sqlite3.RuntimeConfig = wazero.NewRuntimeConfig().WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesThreads)
}

type Store struct {
	db   *sql.DB
	path string
}

// Open opens or creates a SQLite database at dbPath, applies pragmas, sets file permissions 0600,
// and executes all pending schema migrations.
func Open(ctx context.Context, dbPath string) (*Store, error) {
	if dbPath == "" {
		var err error
		dbPath, err = config.DBPath()
		if err != nil {
			return nil, fmt.Errorf("failed to determine db path: %w", err)
		}
	}

	dir := filepath.Dir(dbPath)
	if err := config.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create db parent directory: %w", err)
	}

	// Touch/create DB file with 0600 permissions
	f, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create/open db file: %w", err)
	}
	_ = f.Close()

	if err := os.Chmod(dbPath, 0600); err != nil {
		return nil, fmt.Errorf("failed to set chmod 0600 on db file: %w", err)
	}

	dsn := fmt.Sprintf("file:%s", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Limit idle connections for embedded sqlite
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed executing pragma %q: %w", p, err)
		}
	}

	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed running migrations: %w", err)
	}

	return &Store{
		db:   db,
		path: dbPath,
	}, nil
}

// DB returns the underlying *sql.DB handle.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Path returns the path of the database file.
func (s *Store) Path() string {
	return s.path
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
