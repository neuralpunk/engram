package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Correction represents a stored correction or clarification.
type Correction struct {
	ID         int64
	Fact       string
	Wrong      sql.NullString
	Scope      string
	Tags       sql.NullString
	Source     sql.NullString
	Confidence float64
	CreatedAt  int64
	UpdatedAt  int64
	HitCount   int64
	LastHit    sql.NullInt64
}

// Store inserts a new correction and returns its ID.
func (db *DB) Store(c *Correction) (int64, error) {
	now := time.Now().Unix()
	result, err := db.conn.Exec(
		`INSERT INTO corrections (fact, wrong, scope, tags, source, confidence, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Fact, c.Wrong, c.Scope, c.Tags, c.Source, c.Confidence, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("storing correction: %w", err)
	}
	return result.LastInsertId()
}

// Get retrieves a correction by ID.
func (db *DB) Get(id int64) (*Correction, error) {
	c := &Correction{}
	err := db.conn.QueryRow(
		`SELECT id, fact, wrong, scope, tags, source, confidence, created_at, updated_at, hit_count, last_hit
		 FROM corrections WHERE id = ?`, id,
	).Scan(&c.ID, &c.Fact, &c.Wrong, &c.Scope, &c.Tags, &c.Source,
		&c.Confidence, &c.CreatedAt, &c.UpdatedAt, &c.HitCount, &c.LastHit)
	if err != nil {
		return nil, fmt.Errorf("getting correction %d: %w", id, err)
	}
	return c, nil
}

// UpdateFields specifies which fields to update on a correction.
type UpdateFields struct {
	Fact  *string
	Scope *string
	Tags  *string
}

// Update applies partial updates to a correction.
func (db *DB) Update(id int64, fields UpdateFields) error {
	now := time.Now().Unix()

	// Build dynamic SET clause
	setClauses := "updated_at = ?"
	args := []any{now}

	if fields.Fact != nil {
		setClauses += ", fact = ?"
		args = append(args, *fields.Fact)
	}
	if fields.Scope != nil {
		setClauses += ", scope = ?"
		args = append(args, *fields.Scope)
	}
	if fields.Tags != nil {
		setClauses += ", tags = ?"
		args = append(args, *fields.Tags)
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE corrections SET %s WHERE id = ?", setClauses)

	result, err := db.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("updating correction %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("correction %d not found", id)
	}
	return nil
}

// Delete removes a correction by ID.
func (db *DB) Delete(id int64) error {
	result, err := db.conn.Exec("DELETE FROM corrections WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting correction %d: %w", id, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("correction %d not found", id)
	}
	return nil
}

// List returns corrections filtered by scope and/or tag.
// Pass empty strings to skip filtering. Use limit=0 for no limit.
func (db *DB) List(scope string, tag string, limit int) ([]Correction, error) {
	query := "SELECT id, fact, wrong, scope, tags, source, confidence, created_at, updated_at, hit_count, last_hit FROM corrections WHERE 1=1"
	var args []any

	if scope != "" && scope != "all" {
		query += " AND scope = ?"
		args = append(args, scope)
	}
	if tag != "" {
		query += " AND (',' || tags || ',') LIKE ?"
		args = append(args, "%,"+tag+",%")
	}

	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing corrections: %w", err)
	}
	defer rows.Close()

	var corrections []Correction
	for rows.Next() {
		var c Correction
		if err := rows.Scan(&c.ID, &c.Fact, &c.Wrong, &c.Scope, &c.Tags, &c.Source,
			&c.Confidence, &c.CreatedAt, &c.UpdatedAt, &c.HitCount, &c.LastHit); err != nil {
			return nil, fmt.Errorf("scanning correction: %w", err)
		}
		corrections = append(corrections, c)
	}
	return corrections, rows.Err()
}
