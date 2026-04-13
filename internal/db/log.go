package db

import (
	"fmt"
	"time"
)

// CreateSession records a new session.
func (db *DB) CreateSession(id string, project string) error {
	_, err := db.conn.Exec(
		"INSERT INTO sessions (id, project, created_at) VALUES (?, ?, ?)",
		id, project, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	return nil
}

// LogInjection records that a correction was injected into a session.
func (db *DB) LogInjection(sessionID string, correctionID int64, tokenEstimate int) error {
	_, err := db.conn.Exec(
		"INSERT INTO injection_log (session_id, correction_id, injected_at, token_estimate) VALUES (?, ?, ?, ?)",
		sessionID, correctionID, time.Now().Unix(), tokenEstimate,
	)
	if err != nil {
		return fmt.Errorf("logging injection: %w", err)
	}
	return nil
}

// StatsResult holds aggregate statistics.
type StatsResult struct {
	TotalCorrections int
	TotalInjections  int
	TotalSessions    int
	TypeBreakdown    map[string]int
	StaleCount       int
	TopCorrections   []CorrectionStat
}

// CorrectionStat is a correction with its hit count for the stats view.
type CorrectionStat struct {
	ID       int64
	Fact     string
	Scope    string
	HitCount int64
}

// Stats returns aggregate statistics about corrections and injections.
func (db *DB) Stats() (*StatsResult, error) {
	s := &StatsResult{
		TypeBreakdown: make(map[string]int),
	}

	if err := db.conn.QueryRow("SELECT COUNT(*) FROM corrections").Scan(&s.TotalCorrections); err != nil {
		return nil, fmt.Errorf("counting corrections: %w", err)
	}
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM injection_log").Scan(&s.TotalInjections); err != nil {
		return nil, fmt.Errorf("counting injections: %w", err)
	}
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&s.TotalSessions); err != nil {
		return nil, fmt.Errorf("counting sessions: %w", err)
	}

	// Type breakdown
	typeRows, err := db.conn.Query("SELECT type, COUNT(*) FROM corrections GROUP BY type")
	if err != nil {
		return nil, fmt.Errorf("querying type breakdown: %w", err)
	}
	defer typeRows.Close()
	for typeRows.Next() {
		var t string
		var count int
		if err := typeRows.Scan(&t, &count); err != nil {
			return nil, fmt.Errorf("scanning type breakdown: %w", err)
		}
		s.TypeBreakdown[t] = count
	}
	if err := typeRows.Err(); err != nil {
		return nil, err
	}

	// Stale count
	staleThreshold := time.Now().Unix() - 180*86400
	if err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM corrections WHERE hit_count = 0 OR (last_hit IS NOT NULL AND last_hit < ?)",
		staleThreshold,
	).Scan(&s.StaleCount); err != nil {
		return nil, fmt.Errorf("counting stale corrections: %w", err)
	}

	// Top corrections
	rows, err := db.conn.Query(
		"SELECT id, fact, scope, hit_count FROM corrections ORDER BY hit_count DESC LIMIT 10",
	)
	if err != nil {
		return nil, fmt.Errorf("querying top corrections: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cs CorrectionStat
		if err := rows.Scan(&cs.ID, &cs.Fact, &cs.Scope, &cs.HitCount); err != nil {
			return nil, fmt.Errorf("scanning correction stat: %w", err)
		}
		s.TopCorrections = append(s.TopCorrections, cs)
	}
	return s, rows.Err()
}
