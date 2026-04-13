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

// stopWords are common English words filtered from search queries.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true,
	"but": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "with": true, "by": true, "from": true,
	"is": true, "it": true, "as": true, "be": true, "was": true,
	"are": true, "were": true, "been": true, "have": true, "has": true,
	"do": true, "does": true, "did": true, "this": true, "that": true,
	"i": true, "we": true, "you": true, "not": true, "no": true,
}

func filterStopWords(words []string) []string {
	out := words[:0:0]
	for _, w := range words {
		if !stopWords[strings.ToLower(w)] && len(w) > 1 {
			out = append(out, w)
		}
	}
	return out
}

// Search performs BM25-ranked full-text search with a phrase-first cascade.
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

	// Tier 1: exact phrase match — highest precision
	phraseQuery := `"` + strings.ReplaceAll(query, `"`, ``) + `"`
	if results, err := db.searchFTS(phraseQuery, scopes, limit, minScore); err == nil && len(results) > 0 {
		return results, nil
	}

	// Tier 2: all terms must appear (AND) — good precision, handles reordering
	words := filterStopWords(strings.Fields(query))
	if len(words) > 1 {
		andQuery := buildAndQuery(words)
		if results, err := db.searchFTS(andQuery, scopes, limit, minScore); err == nil && len(results) > 0 {
			return results, nil
		}
	}

	// Tier 3: any term matches (OR) — broadest recall
	if len(words) > 0 {
		orQuery := buildOrQuery(words)
		if results, err := db.searchFTS(orQuery, scopes, limit, minScore); err == nil && len(results) > 0 {
			return results, nil
		}
	}

	// Tier 4: LIKE fallback for single-character/special queries FTS5 rejects
	return db.searchFallback(query, scopes, limit)
}

func buildAndQuery(words []string) string {
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = `"` + w + `"`
	}
	return strings.Join(quoted, " AND ")
}

func buildOrQuery(words []string) string {
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = `"` + w + `"`
	}
	return strings.Join(quoted, " OR ")
}

// searchFTS performs BM25-ranked full-text search via FTS5.
// Column weights: fact×10, wrong×1, tags×5, trigger_hint×3.
func (db *DB) searchFTS(query string, scopes []string, limit int, minScore float64) ([]ScoredCorrection, error) {
	var args []any
	args = append(args, query)

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
		SELECT c.id, c.fact, c.wrong, c.scope, c.tags, c.source,
		       c.type, c.trigger_hint, c.supersedes_id,
		       c.confidence, c.created_at, c.updated_at, c.hit_count, c.last_hit,
		       bm25(corrections_fts, 10, 1, 5, 3) AS score
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
			&sc.Type, &sc.TriggerHint, &sc.SupersedesID,
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

// searchFallback uses LIKE matching with AND logic when FTS5 returns no results.
func (db *DB) searchFallback(query string, scopes []string, limit int) ([]ScoredCorrection, error) {
	words := filterStopWords(strings.Fields(strings.ToLower(query)))
	if len(words) == 0 {
		return nil, nil
	}

	// All words must appear somewhere in fact, wrong, tags, or trigger_hint (AND logic)
	var conditions []string
	var args []any
	for _, w := range words {
		pattern := "%" + w + "%"
		conditions = append(conditions,
			"(LOWER(fact) LIKE ? OR LOWER(COALESCE(wrong,'')) LIKE ? OR LOWER(COALESCE(tags,'')) LIKE ? OR LOWER(COALESCE(trigger_hint,'')) LIKE ?)")
		args = append(args, pattern, pattern, pattern, pattern)
	}
	whereClause := strings.Join(conditions, " AND ")

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
		SELECT id, fact, wrong, scope, tags, source,
		       type, trigger_hint, supersedes_id,
		       confidence, created_at, updated_at, hit_count, last_hit
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
			&sc.Type, &sc.TriggerHint, &sc.SupersedesID,
			&sc.Confidence, &sc.CreatedAt, &sc.UpdatedAt, &sc.HitCount, &sc.LastHit); err != nil {
			return nil, fmt.Errorf("fallback scan: %w", err)
		}
		sc.Score = -1.0
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
