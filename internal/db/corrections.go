package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Correction represents a stored correction or clarification.
type Correction struct {
	ID           int64
	Fact         string
	Wrong        sql.NullString
	Scope        string
	Tags         sql.NullString
	Source       sql.NullString
	Type         string         // 'fact' | 'preference' | 'constraint' | 'gotcha' | 'process'
	TriggerHint  sql.NullString // when to surface this correction
	SupersedesID sql.NullInt64  // ID of the correction this replaces
	Confidence   float64
	CreatedAt    int64
	UpdatedAt    int64
	HitCount     int64
	LastHit      sql.NullInt64
}

// correctionColumns is the standard column list for SELECT queries.
const correctionColumns = `id, fact, wrong, scope, tags, source, type, trigger_hint, supersedes_id, confidence, created_at, updated_at, hit_count, last_hit`

// scanCorrection scans a row into a Correction.
func scanCorrection(scanner interface{ Scan(...any) error }) (*Correction, error) {
	c := &Correction{}
	err := scanner.Scan(&c.ID, &c.Fact, &c.Wrong, &c.Scope, &c.Tags, &c.Source,
		&c.Type, &c.TriggerHint, &c.SupersedesID,
		&c.Confidence, &c.CreatedAt, &c.UpdatedAt, &c.HitCount, &c.LastHit)
	return c, err
}

// Store inserts a new correction and returns its ID.
func (db *DB) Store(c *Correction) (int64, error) {
	now := time.Now().Unix()
	if c.Type == "" {
		c.Type = "fact"
	}
	result, err := db.conn.Exec(
		`INSERT INTO corrections
			(fact, wrong, scope, tags, source, type, trigger_hint, supersedes_id, confidence, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Fact, c.Wrong, c.Scope, c.Tags, c.Source,
		c.Type, c.TriggerHint, c.SupersedesID,
		c.Confidence, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("storing correction: %w", err)
	}
	return result.LastInsertId()
}

// Get retrieves a correction by ID.
func (db *DB) Get(id int64) (*Correction, error) {
	c, err := scanCorrection(db.conn.QueryRow(
		`SELECT `+correctionColumns+` FROM corrections WHERE id = ?`, id,
	))
	if err != nil {
		return nil, fmt.Errorf("getting correction %d: %w", id, err)
	}
	return c, nil
}

// UpdateFields specifies which fields to update on a correction.
type UpdateFields struct {
	Fact        *string
	Scope       *string
	Tags        *string
	Type        *string
	TriggerHint *string
}

// Update applies partial updates to a correction.
func (db *DB) Update(id int64, fields UpdateFields) error {
	now := time.Now().Unix()

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
	if fields.Type != nil {
		setClauses += ", type = ?"
		args = append(args, *fields.Type)
	}
	if fields.TriggerHint != nil {
		setClauses += ", trigger_hint = ?"
		args = append(args, *fields.TriggerHint)
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE corrections SET %s WHERE id = ?", setClauses)

	result, err := db.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("updating correction %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking affected rows: %w", err)
	}
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
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("correction %d not found", id)
	}
	return nil
}

// supersessionFilter is the WHERE clause that excludes corrections that have been
// replaced by another correction via supersedes_id.
const supersessionFilter = " AND id NOT IN (SELECT supersedes_id FROM corrections WHERE supersedes_id IS NOT NULL)"

// List returns corrections filtered by scope and/or tag.
// Pass empty strings to skip filtering. Use limit=0 for no limit.
// If stale is true, returns only corrections not retrieved in 180 days.
func (db *DB) List(scope string, tag string, limit int, stale ...bool) ([]Correction, error) {
	query := "SELECT " + correctionColumns + " FROM corrections WHERE 1=1"
	query += supersessionFilter
	var args []any

	if scope != "" && scope != "all" {
		query += " AND scope = ?"
		args = append(args, scope)
	}
	if tag != "" {
		query += " AND (',' || tags || ',') LIKE ?"
		args = append(args, "%,"+tag+",%")
	}
	if len(stale) > 0 && stale[0] {
		query += " AND (hit_count = 0 OR (last_hit IS NOT NULL AND last_hit < ?))"
		args = append(args, time.Now().Unix()-180*86400)
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
			&c.Type, &c.TriggerHint, &c.SupersedesID,
			&c.Confidence, &c.CreatedAt, &c.UpdatedAt, &c.HitCount, &c.LastHit); err != nil {
			return nil, fmt.Errorf("scanning correction: %w", err)
		}
		corrections = append(corrections, c)
	}
	return corrections, rows.Err()
}

// ListByScopes returns corrections whose scope is in the given set.
// Pass nil or empty for no scope filter. Other parameters match List.
func (db *DB) ListByScopes(scopes []string, tag string, limit int) ([]Correction, error) {
	query := "SELECT " + correctionColumns + " FROM corrections WHERE 1=1"
	query += supersessionFilter
	var args []any

	if len(scopes) > 0 {
		placeholders := make([]string, len(scopes))
		for i, s := range scopes {
			placeholders[i] = "?"
			args = append(args, s)
		}
		query += fmt.Sprintf(" AND scope IN (%s)", strings.Join(placeholders, ","))
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
		return nil, fmt.Errorf("listing corrections by scopes: %w", err)
	}
	defer rows.Close()

	var corrections []Correction
	for rows.Next() {
		var c Correction
		if err := rows.Scan(&c.ID, &c.Fact, &c.Wrong, &c.Scope, &c.Tags, &c.Source,
			&c.Type, &c.TriggerHint, &c.SupersedesID,
			&c.Confidence, &c.CreatedAt, &c.UpdatedAt, &c.HitCount, &c.LastHit); err != nil {
			return nil, fmt.Errorf("scanning correction: %w", err)
		}
		corrections = append(corrections, c)
	}
	return corrections, rows.Err()
}
