package db

import (
	"database/sql"
	"strings"
	"testing"
)

func mustOpenMemory(t *testing.T) *DB {
	t.Helper()
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenMemory(t *testing.T) {
	db := mustOpenMemory(t)

	// Verify all migrations applied
	var version int
	err := db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		t.Fatalf("querying schema version: %v", err)
	}
	if version != 3 {
		t.Errorf("expected version 3, got %d", version)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := mustOpenMemory(t)
	// Running migrate again should not fail
	if err := db.migrate(); err != nil {
		t.Fatalf("second migration failed: %v", err)
	}
}

func TestStoreThenGet(t *testing.T) {
	db := mustOpenMemory(t)

	c := &Correction{
		Fact:       "This project uses BurntSushi/toml, not viper.",
		Wrong:      sql.NullString{String: "viper", Valid: true},
		Scope:      "project:myproject",
		Tags:       sql.NullString{String: "config,go", Valid: true},
		Source:     sql.NullString{String: "user", Valid: true},
		Confidence: 1.0,
	}
	id, err := db.Store(c)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if id < 1 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	got, err := db.Get(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Fact != c.Fact {
		t.Errorf("fact mismatch: got %q", got.Fact)
	}
	if got.Scope != c.Scope {
		t.Errorf("scope mismatch: got %q", got.Scope)
	}
	if got.CreatedAt == 0 {
		t.Error("expected non-zero created_at")
	}
}

func TestUpdate(t *testing.T) {
	db := mustOpenMemory(t)
	id, _ := db.Store(&Correction{Fact: "original", Scope: "global", Confidence: 1.0})

	newFact := "updated fact"
	err := db.Update(id, UpdateFields{Fact: &newFact})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := db.Get(id)
	if got.Fact != "updated fact" {
		t.Errorf("expected updated fact, got %q", got.Fact)
	}
}

func TestDelete(t *testing.T) {
	db := mustOpenMemory(t)
	id, _ := db.Store(&Correction{Fact: "to delete", Scope: "global", Confidence: 1.0})

	if err := db.Delete(id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := db.Get(id)
	if err == nil {
		t.Error("expected error getting deleted correction")
	}
}

func TestDeleteNotFound(t *testing.T) {
	db := mustOpenMemory(t)
	err := db.Delete(9999)
	if err == nil {
		t.Error("expected error deleting nonexistent correction")
	}
}

func TestList(t *testing.T) {
	db := mustOpenMemory(t)
	db.Store(&Correction{Fact: "global fact", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{Fact: "project fact", Scope: "project:foo", Confidence: 1.0})
	db.Store(&Correction{Fact: "another global", Scope: "global", Confidence: 1.0})

	// List all
	all, err := db.List("", "", 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	// List by scope
	globals, err := db.List("global", "", 0)
	if err != nil {
		t.Fatalf("list global: %v", err)
	}
	if len(globals) != 2 {
		t.Errorf("expected 2 globals, got %d", len(globals))
	}

	// List with limit
	limited, err := db.List("", "", 1)
	if err != nil {
		t.Fatalf("list limited: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("expected 1, got %d", len(limited))
	}
}

func TestListByTag(t *testing.T) {
	db := mustOpenMemory(t)
	db.Store(&Correction{
		Fact: "tagged fact", Scope: "global",
		Tags: sql.NullString{String: "go,config", Valid: true}, Confidence: 1.0,
	})
	db.Store(&Correction{
		Fact: "untagged", Scope: "global", Confidence: 1.0,
	})

	results, err := db.List("", "go", 0)
	if err != nil {
		t.Fatalf("list by tag: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 tagged result, got %d", len(results))
	}
}

func TestSearchPhraseFirst(t *testing.T) {
	db := mustOpenMemory(t)
	// Store corrections where phrase match should rank differently than OR match.
	// Note: with tokenchars including '.', trailing periods become part of tokens.
	// Place keywords mid-sentence to avoid this.
	db.Store(&Correction{Fact: "Use config parsing with BurntSushi/toml always.", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{Fact: "Config files are in the root directory.", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{Fact: "Parsing is handled by the parsing module.", Scope: "global", Confidence: 1.0})

	// "config parsing" as a phrase should match the first one best
	results, err := db.Search("config parsing", nil, 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].Fact != "Use config parsing with BurntSushi/toml always." {
		t.Errorf("expected phrase match first, got %q", results[0].Fact)
	}
}

func TestSearchStopWordFiltering(t *testing.T) {
	db := mustOpenMemory(t)
	db.Store(&Correction{Fact: "Authentication requires a JWT token.", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{Fact: "The database uses PostgreSQL.", Scope: "global", Confidence: 1.0})

	// "fix the authentication" — "fix" and "the" should be filtered, "authentication" should match
	results, err := db.Search("fix the authentication", nil, 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results after stop word filtering")
	}
	if results[0].Fact != "Authentication requires a JWT token." {
		t.Errorf("expected auth result, got %q", results[0].Fact)
	}
}

func TestSearchTriggerHint(t *testing.T) {
	db := mustOpenMemory(t)
	db.Store(&Correction{
		Fact:        "Always validate user input before SQL queries.",
		Scope:       "global",
		TriggerHint: sql.NullString{String: "when writing database queries or API handlers", Valid: true},
		Confidence:  1.0,
	})

	results, err := db.Search("database queries API", nil, 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected trigger_hint to be searchable")
	}
}

func TestSearchFactOutranksWrong(t *testing.T) {
	db := mustOpenMemory(t)
	// One correction has "config" in the fact, another has "config" only in wrong
	db.Store(&Correction{
		Fact:       "This project uses toml for config parsing.",
		Scope:      "global",
		Confidence: 1.0,
	})
	db.Store(&Correction{
		Fact:       "Use the standard library for everything.",
		Wrong:      sql.NullString{String: "config parsing was considered but rejected", Valid: true},
		Scope:      "global",
		Confidence: 1.0,
	})

	results, err := db.Search("config", nil, 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Fact match (weight 10) should outrank wrong match (weight 1)
	if results[0].Fact != "This project uses toml for config parsing." {
		t.Errorf("expected fact match first, got %q", results[0].Fact)
	}
}

func TestSearchFTS(t *testing.T) {
	db := mustOpenMemory(t)
	db.Store(&Correction{Fact: "This project uses BurntSushi/toml for configuration.", Scope: "project:foo", Confidence: 1.0})
	db.Store(&Correction{Fact: "Always use snake_case for variable names.", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{Fact: "The database is PostgreSQL 15.", Scope: "project:bar", Confidence: 1.0})

	results, err := db.Search("toml configuration", nil, 10, 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one search result")
	}
	if results[0].Fact != "This project uses BurntSushi/toml for configuration." {
		t.Errorf("expected toml result first, got %q", results[0].Fact)
	}
}

func TestSearchWithScopeFilter(t *testing.T) {
	db := mustOpenMemory(t)
	db.Store(&Correction{Fact: "Use Go 1.22 features only.", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{Fact: "Use Go modules for dependency management.", Scope: "project:foo", Confidence: 1.0})

	results, err := db.Search("Go", []string{"global"}, 10, 0)
	if err != nil {
		t.Fatalf("search with scope: %v", err)
	}
	for _, r := range results {
		if r.Scope != "global" {
			t.Errorf("expected only global results, got scope %q", r.Scope)
		}
	}
}

func TestSearchUpdateThenSearch(t *testing.T) {
	db := mustOpenMemory(t)
	id, _ := db.Store(&Correction{Fact: "Use viper for config.", Scope: "global", Confidence: 1.0})

	// Update the fact
	newFact := "Use BurntSushi/toml for config."
	db.Update(id, UpdateFields{Fact: &newFact})

	// Old query should not match well
	results, err := db.Search("viper", nil, 10, 0)
	if err != nil {
		t.Fatalf("search after update: %v", err)
	}
	for _, r := range results {
		if r.ID == id && r.Fact != newFact {
			t.Error("FTS returned stale fact after update")
		}
	}

	// New query should match
	results, err = db.Search("BurntSushi toml", nil, 10, 0)
	if err != nil {
		t.Fatalf("search for new term: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected result for updated term")
	}
}

func TestSearchDeleteThenSearch(t *testing.T) {
	db := mustOpenMemory(t)
	id, _ := db.Store(&Correction{Fact: "Unique deletable fact about quantum entanglement.", Scope: "global", Confidence: 1.0})

	db.Delete(id)

	results, err := db.Search("quantum entanglement", nil, 10, 0)
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	for _, r := range results {
		if r.ID == id {
			t.Error("deleted correction appeared in search results")
		}
	}
}

func TestIncrementHitCounts(t *testing.T) {
	db := mustOpenMemory(t)
	id, _ := db.Store(&Correction{Fact: "test", Scope: "global", Confidence: 1.0})

	if err := db.IncrementHitCounts([]int64{id}); err != nil {
		t.Fatalf("increment: %v", err)
	}

	got, _ := db.Get(id)
	if got.HitCount != 1 {
		t.Errorf("expected hit_count 1, got %d", got.HitCount)
	}
	if !got.LastHit.Valid {
		t.Error("expected last_hit to be set")
	}
}

func TestNewColumnsExist(t *testing.T) {
	db := mustOpenMemory(t)

	// Verify type, trigger_hint, supersedes_id columns are present
	c := &Correction{
		Fact:         "Test with all new fields.",
		Scope:        "global",
		Type:         "constraint",
		TriggerHint:  sql.NullString{String: "when refactoring", Valid: true},
		SupersedesID: sql.NullInt64{Int64: 0, Valid: false},
		Confidence:   1.0,
	}
	id, err := db.Store(c)
	if err != nil {
		t.Fatalf("store with new fields: %v", err)
	}

	got, err := db.Get(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Type != "constraint" {
		t.Errorf("type: got %q, want %q", got.Type, "constraint")
	}
	if !got.TriggerHint.Valid || got.TriggerHint.String != "when refactoring" {
		t.Errorf("trigger_hint: got %v", got.TriggerHint)
	}
}

func TestStoreWithSupersedes(t *testing.T) {
	db := mustOpenMemory(t)

	id1, _ := db.Store(&Correction{Fact: "old fact", Scope: "global", Confidence: 1.0})
	id2, err := db.Store(&Correction{
		Fact:         "new fact replaces old",
		Scope:        "global",
		SupersedesID: sql.NullInt64{Int64: id1, Valid: true},
		Confidence:   1.0,
	})
	if err != nil {
		t.Fatalf("store with supersedes: %v", err)
	}

	got, _ := db.Get(id2)
	if !got.SupersedesID.Valid || got.SupersedesID.Int64 != id1 {
		t.Errorf("supersedes_id: got %v, want %d", got.SupersedesID, id1)
	}
}

func TestTypeEnumValidation(t *testing.T) {
	db := mustOpenMemory(t)

	// Valid types should succeed
	for _, typ := range []string{"fact", "preference", "constraint", "gotcha", "process"} {
		_, err := db.Store(&Correction{Fact: "test " + typ, Scope: "global", Type: typ, Confidence: 1.0})
		if err != nil {
			t.Errorf("valid type %q failed: %v", typ, err)
		}
	}

	// Invalid type should fail (CHECK constraint)
	_, err := db.Store(&Correction{Fact: "bad type", Scope: "global", Type: "invalid", Confidence: 1.0})
	if err == nil {
		t.Error("expected error for invalid type, got nil")
	}
}

func TestListStale(t *testing.T) {
	db := mustOpenMemory(t)

	// Store a correction with zero hits (should be stale)
	db.Store(&Correction{Fact: "never used", Scope: "global", Confidence: 1.0})

	// Store one and give it hits
	id2, _ := db.Store(&Correction{Fact: "often used", Scope: "global", Confidence: 1.0})
	db.IncrementHitCounts([]int64{id2})

	stale, err := db.List("", "", 0, true)
	if err != nil {
		t.Fatalf("list stale: %v", err)
	}

	// The "never used" one should appear (hit_count=0)
	found := false
	for _, c := range stale {
		if c.Fact == "never used" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'never used' correction in stale list")
	}
}

// §1: Migration runner tests
func TestMigrateAllApplied(t *testing.T) {
	db := mustOpenMemory(t)
	var maxVer int
	if err := db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&maxVer); err != nil {
		t.Fatal(err)
	}
	if maxVer != 3 {
		t.Errorf("expected max version 3, got %d", maxVer)
	}
}

func TestMigrateSecondRunNoop(t *testing.T) {
	db := mustOpenMemory(t)
	// Running migrate again should be a no-op
	if err := db.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var count int
	db.conn.QueryRow("SELECT count(*) FROM schema_version WHERE version=1").Scan(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 row for version=1, got %d", count)
	}
}

// §2: Tokenizer tests
func TestTokenizerIdentifiers(t *testing.T) {
	db := mustOpenMemory(t)

	tests := []struct {
		fact  string
		query string
	}{
		{"This project uses burntsushi/toml for config", "burntsushi/toml"},
		{"Replace use_state with useState in React", "use_state"},
		{"The café service handles i18n", "cafe"},
		{"Use github.com/spf13/cobra for CLI", "cobra"},
		{"Use github.com/spf13/cobra for CLI", "github.com/spf13/cobra"},
	}
	for _, tt := range tests {
		db.Store(&Correction{Fact: tt.fact, Scope: "global", Confidence: 1.0})
	}

	for _, tt := range tests {
		results, err := db.Search(tt.query, nil, 10, 0)
		if err != nil {
			t.Errorf("search %q: %v", tt.query, err)
			continue
		}
		found := false
		for _, r := range results {
			if r.Fact == tt.fact {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("search %q: expected to find %q", tt.query, tt.fact)
		}
	}
}

// §3: Trigram tests — trigram FTS5 does exact substring matching, not fuzzy.
// It handles partial queries and substrings, not typos.
func TestTrigramSubstringMatch(t *testing.T) {
	db := mustOpenMemory(t)
	db.Store(&Correction{Fact: "BurntSushi/toml is the config parser", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{Fact: "Use Postgres for production databases", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{Fact: "The authentication service handles login", Scope: "global", Confidence: 1.0})

	tests := []struct {
		query    string
		contains string
	}{
		{"BurntSush", "BurntSushi"},       // partial match (substring)
		{"Postgre", "Postgres"},           // partial match
		{"authenticat", "authentication"}, // partial match
	}
	for _, tt := range tests {
		results, err := db.Search(tt.query, nil, 10, 0)
		if err != nil {
			t.Errorf("trigram search %q: %v", tt.query, err)
			continue
		}
		found := false
		for _, r := range results {
			if strings.Contains(r.Fact, tt.contains) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("trigram search %q: expected to find correction containing %q", tt.query, tt.contains)
		}
	}
}

// §4: ListByScopes tests
func TestListByScopes(t *testing.T) {
	db := mustOpenMemory(t)
	db.Store(&Correction{Fact: "global fact", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{Fact: "project fact", Scope: "project:foo", Confidence: 1.0})
	db.Store(&Correction{Fact: "domain fact", Scope: "domain:go", Confidence: 1.0})

	results, err := db.ListByScopes([]string{"global", "project:foo"}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	for _, r := range results {
		if r.Scope != "global" && r.Scope != "project:foo" {
			t.Errorf("unexpected scope: %s", r.Scope)
		}
	}
	// Should be in created_at DESC order
	if results[0].CreatedAt < results[1].CreatedAt {
		t.Error("expected created_at DESC order")
	}
}

func TestListByScopesEmpty(t *testing.T) {
	db := mustOpenMemory(t)
	db.Store(&Correction{Fact: "global fact", Scope: "global", Confidence: 1.0})

	results, err := db.ListByScopes(nil, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 (no filter), got %d", len(results))
	}
}

// §5: Supersession tests
func TestSupersessionFilterSearch(t *testing.T) {
	db := mustOpenMemory(t)
	id1, _ := db.Store(&Correction{Fact: "Dev DB is on port 5432", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{
		Fact:         "Dev DB moved to port 5433",
		Scope:        "global",
		SupersedesID: sql.NullInt64{Int64: id1, Valid: true},
		Confidence:   1.0,
	})

	results, err := db.Search("port", nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.ID == id1 {
			t.Error("superseded correction appeared in search results")
		}
	}
}

func TestSupersessionFilterList(t *testing.T) {
	db := mustOpenMemory(t)
	id1, _ := db.Store(&Correction{Fact: "Old fact", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{
		Fact:         "New fact",
		Scope:        "global",
		SupersedesID: sql.NullInt64{Int64: id1, Valid: true},
		Confidence:   1.0,
	})

	list, err := db.List("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range list {
		if c.ID == id1 {
			t.Error("superseded correction appeared in list")
		}
	}
}

func TestSupersessionGetByID(t *testing.T) {
	db := mustOpenMemory(t)
	id1, _ := db.Store(&Correction{Fact: "Old fact", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{
		Fact:         "New fact",
		Scope:        "global",
		SupersedesID: sql.NullInt64{Int64: id1, Valid: true},
		Confidence:   1.0,
	})

	// Get by ID should still return superseded corrections
	got, err := db.Get(id1)
	if err != nil {
		t.Fatalf("Get superseded: %v", err)
	}
	if got.Fact != "Old fact" {
		t.Errorf("expected Old fact, got %q", got.Fact)
	}
}

// §6: Negation preservation tests
func TestNegationNotStripped(t *testing.T) {
	db := mustOpenMemory(t)
	db.Store(&Correction{Fact: "We do not use viper for config", Scope: "global", Confidence: 1.0})

	results, err := db.Search("not viper", nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected to find 'not viper' correction")
	}
}

func TestNegationRanking(t *testing.T) {
	db := mustOpenMemory(t)
	db.Store(&Correction{Fact: "We use viper for config", Scope: "global", Confidence: 1.0})
	db.Store(&Correction{Fact: "We do not use viper for config", Scope: "global", Confidence: 1.0})

	results, err := db.Search("not viper", nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result for 'not viper'")
	}
	// AND query finds only the correction containing BOTH "not" and "viper"
	// The positive version ("We use viper") doesn't contain "not", so it's excluded
	if results[0].Fact != "We do not use viper for config" {
		t.Errorf("expected negation version, got %q", results[0].Fact)
	}
}

func TestSessionAndInjectionLog(t *testing.T) {
	db := mustOpenMemory(t)

	if err := db.CreateSession("sess-1", "myproject"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	id, _ := db.Store(&Correction{Fact: "test", Scope: "global", Confidence: 1.0})
	if err := db.LogInjection("sess-1", id, 25); err != nil {
		t.Fatalf("log injection: %v", err)
	}

	stats, err := db.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TotalCorrections != 1 {
		t.Errorf("expected 1 correction, got %d", stats.TotalCorrections)
	}
	if stats.TotalInjections != 1 {
		t.Errorf("expected 1 injection, got %d", stats.TotalInjections)
	}
	if stats.TotalSessions != 1 {
		t.Errorf("expected 1 session, got %d", stats.TotalSessions)
	}
}
