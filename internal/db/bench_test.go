package db

import (
	"database/sql"
	"fmt"
	"testing"
)

func seedDB(b *testing.B, db *DB, n int) {
	b.Helper()
	scopes := []string{"global", "project:myproject", "domain:go", "domain:python", "project:other"}
	facts := []string{
		"This project uses %s for configuration management.",
		"Always prefer %s over alternatives in this codebase.",
		"The %s module requires special initialization.",
		"Do not use %s in production code.",
		"The %s API changed in the latest version.",
		"Remember to handle %s errors explicitly.",
		"The %s service runs on port 8080.",
		"Use %s for all database migrations.",
		"The %s library has a known memory leak in v2.",
		"Prefer %s for JSON serialization in this project.",
	}
	words := []string{"viper", "toml", "sqlite", "postgres", "redis", "grpc", "http", "auth", "logging", "metrics"}

	for i := 0; i < n; i++ {
		fact := fmt.Sprintf(facts[i%len(facts)], words[i%len(words)])
		scope := scopes[i%len(scopes)]
		tags := fmt.Sprintf("tag%d,tag%d", i%5, i%3)
		db.Store(&Correction{
			Fact:       fact,
			Wrong:      sql.NullString{String: fmt.Sprintf("wrong assumption %d", i), Valid: true},
			Scope:      scope,
			Tags:       sql.NullString{String: tags, Valid: true},
			Source:     sql.NullString{String: "user", Valid: true},
			Confidence: 1.0,
		})
	}
}

func BenchmarkSearch10(b *testing.B) {
	db, _ := OpenMemory()
	defer db.Close()
	seedDB(b, db, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Search("toml configuration", nil, 10, 0)
	}
}

func BenchmarkSearch100(b *testing.B) {
	db, _ := OpenMemory()
	defer db.Close()
	seedDB(b, db, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Search("sqlite database migration", nil, 10, 0)
	}
}

func BenchmarkSearch1000(b *testing.B) {
	db, _ := OpenMemory()
	defer db.Close()
	seedDB(b, db, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Search("postgres grpc service", nil, 10, 0)
	}
}

func BenchmarkSearch10000(b *testing.B) {
	db, _ := OpenMemory()
	defer db.Close()
	seedDB(b, db, 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Search("auth logging metrics", nil, 10, 0)
	}
}

func BenchmarkSearchWithScope10000(b *testing.B) {
	db, _ := OpenMemory()
	defer db.Close()
	seedDB(b, db, 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Search("toml configuration", []string{"global", "project:myproject"}, 10, 0)
	}
}

func BenchmarkStore(b *testing.B) {
	db, _ := OpenMemory()
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Store(&Correction{
			Fact:       fmt.Sprintf("Benchmark fact number %d about testing.", i),
			Scope:      "global",
			Confidence: 1.0,
		})
	}
}

func BenchmarkList(b *testing.B) {
	db, _ := OpenMemory()
	defer db.Close()
	seedDB(b, db, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.List("global", "", 10)
	}
}

func BenchmarkSearchPhraseFirst1000(b *testing.B) {
	db, _ := OpenMemory()
	defer db.Close()
	seedDB(b, db, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Search("toml configuration management", nil, 10, 0)
	}
}

func BenchmarkSearchPhraseFirst10000(b *testing.B) {
	db, _ := OpenMemory()
	defer db.Close()
	seedDB(b, db, 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Search("toml configuration management", nil, 10, 0)
	}
}
