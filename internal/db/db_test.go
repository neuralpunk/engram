package db

import (
	"database/sql"
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

	// Verify schema_version was set
	var version int
	err := db.conn.QueryRow("SELECT version FROM schema_version").Scan(&version)
	if err != nil {
		t.Fatalf("querying schema version: %v", err)
	}
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
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
