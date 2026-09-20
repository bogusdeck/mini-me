package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type FactStatus string

const (
	FactStatusProposed  FactStatus = "proposed"
	FactStatusConfirmed FactStatus = "confirmed"
	FactStatusRejected  FactStatus = "rejected"
)

type FactSensitivity string

const (
	SensitivityNormal     FactSensitivity = "normal"
	SensitivityPersonal   FactSensitivity = "personal"
	SensitivityNeverInfer FactSensitivity = "never_infer"
)

type Fact struct {
	ID           int64           `json:"id"`
	SubjectID    int64           `json:"subject_id"`
	Predicate    string          `json:"predicate"`
	Object       string          `json:"object"`
	Status       FactStatus      `json:"status"`
	Confidence   float64         `json:"confidence"`
	Sensitivity  FactSensitivity `json:"sensitivity"`
	ValidFrom    *time.Time      `json:"valid_from,omitempty"`
	ValidTo      *time.Time      `json:"valid_to,omitempty"`
	SupersededBy *int64          `json:"superseded_by,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

func (s *Store) CreateFact(ctx context.Context, f *Fact) (int64, error) {
	if f.Confidence == 0 {
		f.Confidence = 1.0
	}
	if f.Status == "" {
		f.Status = FactStatusProposed
	}
	if f.Sensitivity == "" {
		f.Sensitivity = SensitivityNormal
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO fact (subject_id, predicate, object, status, confidence, sensitivity, valid_from, valid_to, superseded_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, f.SubjectID, f.Predicate, f.Object, f.Status, f.Confidence, f.Sensitivity, f.ValidFrom, f.ValidTo, f.SupersededBy)
	if err != nil {
		return 0, fmt.Errorf("failed creating fact: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed getting fact insert id: %w", err)
	}
	f.ID = id
	return id, nil
}

func (s *Store) UpdateFactStatus(ctx context.Context, id int64, status FactStatus) error {
	_, err := s.db.ExecContext(ctx, "UPDATE fact SET status = ? WHERE id = ?", status, id)
	if err != nil {
		return fmt.Errorf("failed updating fact status: %w", err)
	}
	return nil
}

func (s *Store) LinkFactSource(ctx context.Context, factID, chunkID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO fact_source (fact_id, chunk_id) VALUES (?, ?)
	`, factID, chunkID)
	if err != nil {
		return fmt.Errorf("failed linking fact to chunk source: %w", err)
	}
	return nil
}

func (s *Store) ListFactsByStatus(ctx context.Context, status FactStatus) ([]*Fact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, subject_id, predicate, object, status, confidence, sensitivity, valid_from, valid_to, superseded_by, created_at
		FROM fact WHERE status = ? ORDER BY created_at DESC
	`, status)
	if err != nil {
		return nil, fmt.Errorf("failed querying facts by status: %w", err)
	}
	defer rows.Close()

	var list []*Fact
	for rows.Next() {
		f := &Fact{}
		var vf, vt sql.NullTime
		var sb sql.NullInt64
		if err := rows.Scan(&f.ID, &f.SubjectID, &f.Predicate, &f.Object, &f.Status, &f.Confidence, &f.Sensitivity, &vf, &vt, &sb, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed scanning fact row: %w", err)
		}
		if vf.Valid {
			f.ValidFrom = &vf.Time
		}
		if vt.Valid {
			f.ValidTo = &vt.Time
		}
		if sb.Valid {
			f.SupersededBy = &sb.Int64
		}
		list = append(list, f)
	}
	return list, rows.Err()
}
