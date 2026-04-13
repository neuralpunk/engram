package db

import (
	"fmt"
	"strings"
	"time"
)

// ScoredCorrection is a Correction with its BM25 relevance score.
type ScoredCorrection struct {
	Correction
	Score float64
}

// Search performs BM25-ranked full-text search over corrections.
// scopes filters results to matching scope values. Pass nil or empty for no scope filter.
// minScore filters out results below this BM25 relevance threshold.
// Falls back to LIKE-based matching if FTS5 returns no results.
func (db *DB) Search(query string, scopes []string, limit int, minScore float64) ([]ScoredCorrection, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}

	// Try FTS5 BM25 search first
	results, err := db.searchFTS(query, scopes, limit, minScore)
	if err == nil && len(results) > 0 {
		return results, nil
	}

	// Fallback: LIKE-based search for small corpora or when FTS5 misses
	return db.searchFallback(query, scopes, limit)
}

// searchFTS performs BM25-ranked full-text search via FTS5.
func (db *DB) searchFTS(query string, scopes []string, limit int, minScore float64) ([]ScoredCorrection, error) {
	var args []any

	// Build FTS5 query: split into words, join with OR for broad matching
	words := strings.Fields(query)
	for i, w := range words {
		w = strings.ReplaceAll(w, "\"", "")
		if w != "" {
			words[i] = "\"" + w + "\""
		}
	}
	ftsQuery := strings.Join(words, " OR ")
	args = append(args, ftsQuery)

	// Build scope filter
	scopeFilter := ""
	if len(scopes) > 0 {
		placeholders := make([]string, len(scopes))
		for i, s := range scopes {
			placeholders[i] = "?"
			args = append(args, s)
		}
		scopeFilter = fmt.Sprintf(" AND c.scope IN (%s)", strings.Join(placeholders, ","))
	}

	args = append(args, limit)

	sql := fmt.Sprintf(`
		SELECT c.id, c.fact, c.wrong, c.scope, c.tags, c.source, c.confidence,
		       c.created_at, c.updated_at, c.hit_count, c.last_hit,
		       bm25(corrections_fts) AS score
		FROM corrections_fts fts
		JOIN corrections c ON c.id = fts.rowid
		WHERE corrections_fts MATCH ?
		%s
		ORDER BY score
		LIMIT ?
	`, scopeFilter)

	rows, err := db.conn.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ScoredCorrection
	for rows.Next() {
		var sc ScoredCorrection
		if err := rows.Scan(&sc.ID, &sc.Fact, &sc.Wrong, &sc.Scope, &sc.Tags, &sc.Source,
			&sc.Confidence, &sc.CreatedAt, &sc.UpdatedAt, &sc.HitCount, &sc.LastHit,
			&sc.Score); err != nil {
			return nil, err
		}
		if -sc.Score >= minScore {
			results = append(results, sc)
		}
	}
	return results, rows.Err()
}

// searchFallback uses LIKE matching when FTS5 returns no results.
// Matches corrections where any query word appears in fact, wrong, or tags.
func (db *DB) searchFallback(query string, scopes []string, limit int) ([]ScoredCorrection, error) {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil, nil
	}

	// Build WHERE clause: each word must appear in fact, wrong, or tags
	var conditions []string
	var args []any
	for _, w := range words {
		pattern := "%" + w + "%"
		conditions = append(conditions, "(LOWER(fact) LIKE ? OR LOWER(COALESCE(wrong,'')) LIKE ? OR LOWER(COALESCE(tags,'')) LIKE ?)")
		args = append(args, pattern, pattern, pattern)
	}

	// Any word matching is sufficient (OR)
	whereClause := strings.Join(conditions, " OR ")

	// Scope filter
	if len(scopes) > 0 {
		placeholders := make([]string, len(scopes))
		for i, s := range scopes {
			placeholders[i] = "?"
			args = append(args, s)
		}
		whereClause = fmt.Sprintf("(%s) AND scope IN (%s)", whereClause, strings.Join(placeholders, ","))
	}

	args = append(args, limit)

	sql := fmt.Sprintf(`
		SELECT id, fact, wrong, scope, tags, source, confidence,
		       created_at, updated_at, hit_count, last_hit
		FROM corrections
		WHERE %s
		ORDER BY hit_count DESC, updated_at DESC
		LIMIT ?
	`, whereClause)

	rows, err := db.conn.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("fallback search: %w", err)
	}
	defer rows.Close()

	var results []ScoredCorrection
	for rows.Next() {
		var sc ScoredCorrection
		if err := rows.Scan(&sc.ID, &sc.Fact, &sc.Wrong, &sc.Scope, &sc.Tags, &sc.Source,
			&sc.Confidence, &sc.CreatedAt, &sc.UpdatedAt, &sc.HitCount, &sc.LastHit); err != nil {
			return nil, fmt.Errorf("fallback scan: %w", err)
		}
		sc.Score = -1.0 // synthetic score for fallback results
		results = append(results, sc)
	}
	return results, rows.Err()
}

// IncrementHitCounts updates hit_count and last_hit for the given correction IDs.
func (db *DB) IncrementHitCounts(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now().Unix()
	placeholders := make([]string, len(ids))
	args := []any{now}
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := fmt.Sprintf(
		"UPDATE corrections SET hit_count = hit_count + 1, last_hit = ? WHERE id IN (%s)",
		strings.Join(placeholders, ","),
	)
	_, err := db.conn.Exec(query, args...)
	return err
}
