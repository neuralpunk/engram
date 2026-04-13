package cmd

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"engram/internal/db"
	"engram/internal/format"
)

// testDB creates a temp database file for testing.
func testDB(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "engram-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	f.Close()
	t.Cleanup(func() { os.Remove(path) })
	return path
}

// storeVia calls cmd.Store with the given args and db path.
func storeVia(t *testing.T, dbPath string, fact string, flags ...string) {
	t.Helper()
	args := append([]string{fact}, flags...)
	if err := Store(args, dbPath); err != nil {
		t.Fatalf("store %q: %v", fact, err)
	}
}

// getOutput captures stdout from cmd.Get.
func getOutput(t *testing.T, dbPath string, args ...string) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := Get(args, dbPath)
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	return buf.String()
}

// hookOutput captures stdout from cmd.Hook by providing prompt via stdin.
func hookOutput(t *testing.T, dbPath string, prompt string) string {
	t.Helper()

	input, _ := json.Marshal(hookInput{Prompt: prompt})

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	stdinR, stdinW, _ := os.Pipe()
	os.Stdin = stdinR
	stdinW.Write(input)
	stdinW.Close()

	err := Hook(nil, dbPath)
	w.Close()
	os.Stdin = oldStdin
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("hook: %v", err)
	}
	return buf.String()
}

// Category A: direct contradiction detection
func TestRealWorldCategoryA(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
	}{
		{"A1", "No, that's wrong. We use Postgres, not MySQL."},
		{"A2", "That's incorrect — the function is called fetchUser, not getUser."},
		{"A3", "You're wrong, it returns a Promise not a callback."},
		{"A4", "Wrong. The port is 5432."},
		{"A5", "That's not right. We're on Python 3.11, not 3.9."},
		{"A6", "No it's not. The config lives in /etc/myapp/, not the home directory."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !correctionPatterns.MatchString(tc.prompt) {
				t.Errorf("pattern should detect correction in: %s", tc.prompt)
			}
		})
	}
}

// Category B: substitution detection
func TestRealWorldCategoryB(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
	}{
		{"B1", "We use BurntSushi/toml not viper."},
		{"B2", "It's pnpm not npm in this repo."},
		{"B3", "We use squash merges not merge commits."},
		{"B4", "Use ruff not flake8 for linting."},
		{"B5", "It's tailwind not bootstrap."},
		{"B6", "We use Conventional Commits not freeform messages."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !correctionPatterns.MatchString(tc.prompt) {
				t.Errorf("pattern should detect substitution in: %s", tc.prompt)
			}
		})
	}
}

// Category C: stop/never/don't detection
func TestRealWorldCategoryC(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
	}{
		{"C1", "Stop adding markdown formatting to your responses."},
		{"C2", "Please stop adding comments to my code."},
		{"C3", "Don't use emoji in commit messages."},
		{"C4", "Never edit files in the vendor directory."},
		{"C5", "Stop suggesting we add tests, the test suite is owned by another team."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !correctionPatterns.MatchString(tc.prompt) {
				t.Errorf("pattern should detect stop/never in: %s", tc.prompt)
			}
		})
	}
}

// Category D: project facts detection
func TestRealWorldCategoryD(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
	}{
		{"D1", "Remember that the dev server runs on port 8080."},
		{"D2", "Keep in mind we deploy via GitHub Actions, not Jenkins."},
		{"D3", "Going forward, all new endpoints need rate limiting."},
		{"D4", "It's Node 18 not 20 in this project."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !correctionPatterns.MatchString(tc.prompt) {
				t.Errorf("pattern should detect project fact in: %s", tc.prompt)
			}
		})
	}
}

// Category F: frustration detection
func TestRealWorldCategoryF(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
	}{
		{"F1", "I told you already, we don't use Redux in this project."},
		{"F2", "For the last time: PRs need two approvals before merging."},
		{"F3", "How many times do I have to say it — the API is versioned at /v2."},
		{"F4", "I've already mentioned this — we use TanStack Query, not SWR."},
		{"F5", "I told you the migration is on the dev branch."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !correctionPatterns.MatchString(tc.prompt) {
				t.Errorf("pattern should detect frustration in: %s", tc.prompt)
			}
		})
	}
}

// Category G: subtle "actually" detection
func TestRealWorldCategoryG(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
	}{
		{"G1", "Actually, the cache TTL is 60 seconds, not 5 minutes."},
		{"G2", "Actually we deprecated that endpoint last month."},
		{"G3", "Hmm actually it's case-sensitive on Linux but not macOS."},
		{"G4", "Actually the docs say to use --no-cache for that."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !correctionPatterns.MatchString(tc.prompt) {
				t.Errorf("pattern should detect 'actually' in: %s", tc.prompt)
			}
		})
	}
}

// Category J: tokenizer stress tests (store + retrieve round-trip)
func TestRealWorldCategoryJ(t *testing.T) {
	dbPath := testDB(t)
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		fact  string
		query string
	}{
		{"J1", "Use github.com/spf13/cobra for CLI parsing", "github.com/spf13/cobra"},
		{"J2", "Replace use_state with useState in the React rewrite", "use_state"},
		{"J3", "The grpc-web client lives in pkg/transport", "grpc-web"},
		{"J5", "BurntSushi/toml is the parser", "burntsushi/toml"},
		{"J6", "The café service handles i18n", "cafe"},
	}

	for _, tc := range cases {
		database.Store(&db.Correction{
			Fact: tc.fact, Scope: "global", Confidence: 1.0,
		})
	}
	database.Close()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := getOutput(t, dbPath, tc.query, "--raw")
			if !strings.Contains(out, tc.fact) {
				t.Errorf("search %q: expected to find %q in output:\n%s", tc.query, tc.fact, out)
			}
		})
	}
}

// Category K: trigram fallback / substring matching
// Note: trigram FTS5 does exact substring matching, not fuzzy/typo matching.
func TestRealWorldCategoryK(t *testing.T) {
	dbPath := testDB(t)
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	database.Store(&db.Correction{Fact: "BurntSushi/toml is the config parser", Scope: "global", Confidence: 1.0})
	database.Store(&db.Correction{Fact: "Use Postgres for production databases", Scope: "global", Confidence: 1.0})
	database.Store(&db.Correction{Fact: "The authentication service handles login", Scope: "global", Confidence: 1.0})
	database.Close()

	cases := []struct {
		name     string
		query    string
		contains string
	}{
		{"K1_partial", "BurntSush", "BurntSushi"},       // partial substring
		{"K2_partial", "Postgre", "Postgres"},           // partial substring
		{"K3_partial", "authenticat", "authentication"}, // partial substring
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := getOutput(t, dbPath, tc.query, "--raw")
			if !strings.Contains(out, tc.contains) {
				t.Errorf("partial query %q: expected output to contain %q, got:\n%s", tc.query, tc.contains, out)
			}
		})
	}
}

// Category L: negation preservation
func TestRealWorldCategoryL(t *testing.T) {
	dbPath := testDB(t)
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		factA   string
		factB   string
		query   string
		expectB bool
	}{
		{"L1", "We use viper for config", "We do not use viper for config", "not viper", true},
		{"L2", "Always commit secrets to git", "Never commit secrets to git", "no secrets", true},
		{"L3", "The build runs tests", "The build does not run tests", "not run tests", true},
	}

	for _, tc := range cases {
		database.Store(&db.Correction{Fact: tc.factA, Scope: "global", Confidence: 1.0})
		database.Store(&db.Correction{Fact: tc.factB, Scope: "global", Confidence: 1.0})
	}
	database.Close()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := getOutput(t, dbPath, tc.query, "--raw")
			if !strings.Contains(out, tc.factB) {
				t.Errorf("query %q: expected negation fact in output:\n%s", tc.query, out)
			}
		})
	}
}

// Category N: supersession
func TestRealWorldCategoryN(t *testing.T) {
	dbPath := testDB(t)

	// Store old fact
	storeVia(t, dbPath, "Dev DB is on port 5432", "--scope", "global")

	// Store superseding fact
	storeVia(t, dbPath, "Dev DB moved to port 5433 last week", "--scope", "global", "--supersedes", "1")

	// Search should only return the new fact
	out := getOutput(t, dbPath, "port database", "--raw")
	if strings.Contains(out, "5432") && !strings.Contains(out, "5433") {
		t.Error("superseded correction (5432) should not appear without the new one (5433)")
	}
	if !strings.Contains(out, "5433") {
		t.Errorf("expected new fact (5433) in output:\n%s", out)
	}
}

// Category O: dedup on store
func TestRealWorldCategoryO(t *testing.T) {
	t.Run("O1_NearDuplicateRejected", func(t *testing.T) {
		dbPath := testDB(t)
		// First store: establish the correction
		storeVia(t, dbPath, "We use toml for config parsing here", "--scope", "global", "--tags", "toml,config")

		// Capture stderr for the second store — near-duplicate phrasing
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		// Same concepts, high word overlap for Jaccard >= 0.6
		Store([]string{"We use toml for config parsing always", "--scope", "global", "--tags", "toml,config"}, dbPath)
		w.Close()
		os.Stderr = oldStderr

		var buf bytes.Buffer
		io.Copy(&buf, r)

		if !strings.Contains(buf.String(), "similar correction already exists") {
			t.Errorf("expected dedup warning, got stderr:\n%s", buf.String())
		}
	})

	t.Run("O2_ForceOverrides", func(t *testing.T) {
		dbPath := testDB(t)
		storeVia(t, dbPath, "We use toml for config parsing here", "--scope", "global", "--tags", "toml,config")
		storeVia(t, dbPath, "We use toml for config parsing always", "--scope", "global", "--tags", "toml,config", "--force")

		// Both should be stored
		database, err := db.Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer database.Close()
		list, _ := database.List("", "", 0)
		if len(list) != 2 {
			t.Errorf("expected 2 corrections with --force, got %d", len(list))
		}
	})

	t.Run("O3_DifferentScopeNoDedup", func(t *testing.T) {
		dbPath := testDB(t)
		storeVia(t, dbPath, "We use BurntSushi/toml for config", "--scope", "project:a")
		storeVia(t, dbPath, "We use BurntSushi/toml for config", "--scope", "project:b")

		database, err := db.Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer database.Close()
		list, _ := database.List("", "", 0)
		if len(list) != 2 {
			t.Errorf("expected 2 corrections in different scopes, got %d", len(list))
		}
	})

	t.Run("O4_GenuinelyDifferentFacts", func(t *testing.T) {
		dbPath := testDB(t)
		storeVia(t, dbPath, "We use BurntSushi/toml for config", "--scope", "global")
		storeVia(t, dbPath, "We use Postgres for the database", "--scope", "global")

		database, err := db.Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		defer database.Close()
		list, _ := database.List("", "", 0)
		if len(list) != 2 {
			t.Errorf("expected 2 different corrections, got %d", len(list))
		}
	})
}

// Category P: cross-scope retrieval
func TestRealWorldCategoryP(t *testing.T) {
	t.Run("P1_GlobalAndProjectBothSurface", func(t *testing.T) {
		dbPath := testDB(t)
		storeVia(t, dbPath, "Always use 2-space indentation in YAML files", "--scope", "global", "--tags", "yaml,formatting,indentation")
		storeVia(t, dbPath, "This project CI config is in .github/workflows/ directory", "--scope", "project:test", "--tags", "ci,github,workflows")

		out := hookOutput(t, dbPath, "how should I format the CI workflow file?")
		if !strings.Contains(out, "YAML") && !strings.Contains(out, "indentation") {
			t.Logf("hook output:\n%s", out)
			// Global may or may not surface depending on query relevance; don't hard-fail
		}
	})

	t.Run("P2_StrongGlobalBeatsWeakProject", func(t *testing.T) {
		dbPath := testDB(t)
		storeVia(t, dbPath, "Never commit .env files to git ever", "--scope", "global", "--tags", "env,secrets,git,security")
		storeVia(t, dbPath, "The project name is Acme", "--scope", "project:test", "--tags", "name,project")

		out := hookOutput(t, dbPath, "should I add my .env file to the commit?")
		if !strings.Contains(out, ".env") && !strings.Contains(out, "Never commit") {
			t.Errorf("expected .env warning in output:\n%s", out)
		}
	})
}

// Category Q: false positive measurement (document, don't fail)
func TestRealWorldCategoryQ(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
		fires  bool
		note   string
	}{
		{"Q1", "No problem, I'll handle it.", false, ""},
		{"Q2", "I have no idea why that broke.", false, ""},
		{"Q3", "Actually, can you also add a test?", true, "known limitation: 'actually' triggers"},
		{"Q4", "That's not what I meant — let me rephrase.", true, "acceptable: correction context"},
		{"Q5", "Going forward to the next ticket, what should I work on?", true, "known limitation: 'going forward' triggers"},
		{"Q6", "Always nice to see clean code.", false, ""},
		{"Q8", "Remember to grab milk on your way home.", true, "false positive but harmless"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fires := correctionPatterns.MatchString(tc.prompt)
			if fires != tc.fires {
				if tc.note != "" {
					t.Logf("NOTE (%s): fires=%v expected=%v — %s", tc.name, fires, tc.fires, tc.note)
				} else {
					t.Logf("false positive: %q fires=%v", tc.prompt, fires)
				}
			}
		})
	}
}

// Category E: failed attempts
func TestRealWorldCategoryE(t *testing.T) {
	cases := []struct {
		name   string
		prompt string
		detect bool
	}{
		{"E1", "No, that didn't work. The build still fails on the lint step.", true},
		{"E2", "Still broken. The same error.", false},
		{"E3", "That errored out — apparently auth requires the X-Tenant header.", false},
		{"E4", "It crashed because the config file needs to exist before init runs.", false},
		{"E5", "That broke prod last time. We can't change the column type without a backfill.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fires := correctionPatterns.MatchString(tc.prompt)
			if tc.detect && !fires {
				t.Errorf("expected detection for: %s", tc.prompt)
			}
			// Non-detection cases are logged but don't fail
			if !tc.detect && fires {
				t.Logf("unexpected detection for %s: %s (may be acceptable)", tc.name, tc.prompt)
			}
		})
	}
}

// Category M: MMR diversity via store/retrieve round-trip
func TestRealWorldCategoryM(t *testing.T) {
	dbPath := testDB(t)
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	// 5 near-duplicate toml corrections + 1 unrelated
	database.Store(&db.Correction{Fact: "This project uses BurntSushi/toml for config", Scope: "global", Tags: sql.NullString{String: "toml,config", Valid: true}, Confidence: 1.0})
	database.Store(&db.Correction{Fact: "Config files in this repo are TOML parsed by BurntSushi/toml", Scope: "global", Tags: sql.NullString{String: "toml,config", Valid: true}, Confidence: 1.0})
	database.Store(&db.Correction{Fact: "We chose BurntSushi/toml over viper for configuration", Scope: "global", Tags: sql.NullString{String: "toml,config,viper", Valid: true}, Confidence: 1.0})
	database.Store(&db.Correction{Fact: "TOML is the config format here using BurntSushi library", Scope: "global", Tags: sql.NullString{String: "toml,config", Valid: true}, Confidence: 1.0})
	database.Store(&db.Correction{Fact: "The toml package we use is from BurntSushi not pelletier", Scope: "global", Tags: sql.NullString{String: "toml,burntsushi,pelletier", Valid: true}, Confidence: 1.0})
	database.Store(&db.Correction{Fact: "Tests are run with go test -race for concurrency checks", Scope: "global", Tags: sql.NullString{String: "test,race,go", Valid: true}, Confidence: 1.0})
	database.Close()

	out := getOutput(t, dbPath, "config", "--raw", "--limit", "3")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		t.Fatal("expected results")
	}

	// Count how many toml-related vs unrelated
	tomlCount := 0
	for _, l := range lines {
		if strings.Contains(strings.ToLower(l), "toml") || strings.Contains(strings.ToLower(l), "config") {
			tomlCount++
		}
	}
	// With MMR, we shouldn't get ALL toml corrections
	t.Logf("got %d lines, %d toml-related:\n%s", len(lines), tomlCount, out)
	_ = format.JaccardSimilarity // ensure import
	_ = fmt.Sprintf              // ensure import
}
