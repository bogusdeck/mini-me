package store

import (
	"context"
	"database/sql"
	"fmt"
)

type EntityType string

const (
	EntityPerson  EntityType = "person"
	EntityOrg     EntityType = "org"
	EntityProject EntityType = "project"
	EntityTopic   EntityType = "topic"
	EntityPlace   EntityType = "place"
)

type Entity struct {
	ID      int64      `json:"id"`
	Type    EntityType `json:"type"`
	Name    string     `json:"name"`
	Aliases string     `json:"aliases"`
	Notes   string     `json:"notes"`
}

func (s *Store) CreateEntity(ctx context.Context, e *Entity) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO entity (type, name, aliases, notes)
		VALUES (?, ?, ?, ?)
	`, e.Type, e.Name, e.Aliases, e.Notes)
	if err != nil {
		return 0, fmt.Errorf("failed to create entity: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed getting entity last insert id: %w", err)
	}
	e.ID = id
	return id, nil
}

func (s *Store) GetEntityByName(ctx context.Context, name string) (*Entity, error) {
	e := &Entity{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, type, name, aliases, notes
		FROM entity WHERE name = ?
	`, name).Scan(&e.ID, &e.Type, &e.Name, &e.Aliases, &e.Notes)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed querying entity by name: %w", err)
	}
	return e, nil
}

func (s *Store) GetEntityByID(ctx context.Context, id int64) (*Entity, error) {
	e := &Entity{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, type, name, aliases, notes
		FROM entity WHERE id = ?
	`, id).Scan(&e.ID, &e.Type, &e.Name, &e.Aliases, &e.Notes)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed querying entity by id: %w", err)
	}
	return e, nil
}

func (s *Store) ListEntities(ctx context.Context) ([]*Entity, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, name, aliases, notes FROM entity ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed listing entities: %w", err)
	}
	defer rows.Close()

	var list []*Entity
	for rows.Next() {
		e := &Entity{}
		if err := rows.Scan(&e.ID, &e.Type, &e.Name, &e.Aliases, &e.Notes); err != nil {
			return nil, fmt.Errorf("failed scanning entity row: %w", err)
		}
		list = append(list, e)
	}
	return list, rows.Err()
}
