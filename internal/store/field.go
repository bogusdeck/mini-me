package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Field struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Store) SetField(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO field (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`, key, value)
	if err != nil {
		return fmt.Errorf("failed setting field %s: %w", key, err)
	}
	return nil
}

func (s *Store) GetField(ctx context.Context, key string) (*Field, error) {
	f := &Field{}
	err := s.db.QueryRowContext(ctx, `
		SELECT key, value, updated_at FROM field WHERE key = ?
	`, key).Scan(&f.Key, &f.Value, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed getting field %s: %w", key, err)
	}
	return f, nil
}

func (s *Store) ListFields(ctx context.Context) ([]*Field, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT key, value, updated_at FROM field ORDER BY key ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed listing fields: %w", err)
	}
	defer rows.Close()

	var list []*Field
	for rows.Next() {
		f := &Field{}
		if err := rows.Scan(&f.Key, &f.Value, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed scanning field row: %w", err)
		}
		list = append(list, f)
	}
	return list, rows.Err()
}

func (s *Store) DeleteField(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM field WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("failed deleting field %s: %w", key, err)
	}
	return nil
}
