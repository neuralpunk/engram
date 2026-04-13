package format

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"engram/internal/db"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"abcd", 1},
		{"hello world!!", 3},
	}
	for _, tt := range tests {
		got := EstimateTokens(tt.input)
		if got != tt.want {
			t.Errorf("EstimateTokens(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestFormatMemoryBlockEmpty(t *testing.T) {
	out := FormatMemoryBlock(nil)
	if !strings.Contains(out, "No stored corrections") {
		t.Errorf("expected empty message, got %q", out)
	}
}

func TestFormatMemoryBlock(t *testing.T) {
	corrections := []db.Correction{
		{Scope: "global", Fact: "Prefer prose over bullet points.", Type: "fact"},
		{Scope: "project:foo", Fact: "Uses BurntSushi/toml.", Type: "fact"},
	}
	out := FormatMemoryBlock(corrections)

	if !strings.Contains(out, "<engram-memory>") {
		t.Error("missing opening tag")
	}
	if !strings.Contains(out, "</engram-memory>") {
		t.Error("missing closing tag")
	}
	if !strings.Contains(out, "FACTS:") {
		t.Errorf("missing FACTS header in output: %s", out)
	}
	if !strings.Contains(out, "Prefer prose over bullet points.") {
		t.Errorf("missing first correction in output: %s", out)
	}
	if !strings.Contains(out, "Uses BurntSushi/toml.") {
		t.Errorf("missing second correction in output: %s", out)
	}
	// wrong field must NOT appear
	if strings.Contains(out, "wrong") {
		t.Error("wrong field should not appear in memory block")
	}
}

func TestFormatSystemPrompt(t *testing.T) {
	corrections := []db.Correction{
		{Scope: "global", Fact: "Test fact."},
	}
	out := FormatSystemPrompt(corrections)
	if !strings.Contains(out, "<engram>") {
		t.Error("missing behavior prompt")
	}
	if !strings.Contains(out, "<engram-memory>") {
		t.Error("missing memory block")
	}
	if !strings.Contains(out, "engram store") {
		t.Error("missing CLI command reference in behavior prompt")
	}
}

func TestSelectCorrectionsGlobalBonusApplied(t *testing.T) {
	// Global gets 1.2 tier bonus, no-project corrections get 1.0.
	// With equal BM25, global should rank above unmatched project scopes.
	all := []db.ScoredCorrection{
		{Correction: db.Correction{ID: 1, Fact: "alpha bravo charlie", Scope: "project:foo", Confidence: 1.0}, Score: -3.0},
		{Correction: db.Correction{ID: 2, Fact: "delta echo foxtrot", Scope: "global", Confidence: 1.0}, Score: -3.0},
		{Correction: db.Correction{ID: 3, Fact: "golf hotel india", Scope: "project:bar", Confidence: 1.0}, Score: -3.0},
	}

	selected := SelectCorrections(all, 10, 3000, "")
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected, got %d", len(selected))
	}
	// Global (1.2 bonus) should come before non-matched projects (1.0 bonus)
	if selected[0].Scope != "global" {
		t.Errorf("expected global first, got %q", selected[0].Scope)
	}
}

func TestSelectCorrectionsMaxCount(t *testing.T) {
	all := []db.ScoredCorrection{
		{Correction: db.Correction{ID: 1, Fact: "alpha bravo charlie", Scope: "global", Confidence: 1.0}, Score: -5.0},
		{Correction: db.Correction{ID: 2, Fact: "delta echo foxtrot", Scope: "global", Confidence: 1.0}, Score: -3.0},
		{Correction: db.Correction{ID: 3, Fact: "golf hotel india", Scope: "global", Confidence: 1.0}, Score: -4.0},
	}

	selected := SelectCorrections(all, 2, 3000, "")
	if len(selected) != 2 {
		t.Errorf("expected 2 selected, got %d", len(selected))
	}
}

func TestSelectCorrectionsTokenBudget(t *testing.T) {
	// Each entry will be roughly "[global] short" = 15 chars = 3 tokens
	all := []db.ScoredCorrection{
		{Correction: db.Correction{ID: 1, Fact: "short", Scope: "global", Confidence: 1.0}, Score: -5.0},
		{Correction: db.Correction{ID: 2, Fact: strings.Repeat("x", 1200), Scope: "global", Confidence: 1.0}, Score: -3.0}, // ~300 tokens
	}

	selected := SelectCorrections(all, 10, 50, "") // tight budget
	if len(selected) != 1 {
		t.Errorf("expected 1 within budget, got %d", len(selected))
	}
}

func TestConstraintsRenderFirst(t *testing.T) {
	corrections := []db.Correction{
		{Scope: "global", Fact: "A regular fact.", Type: "fact"},
		{Scope: "global", Fact: "Never delete the queue.", Type: "constraint"},
		{Scope: "global", Fact: "Prefer prose.", Type: "preference"},
	}
	out := FormatMemoryBlock(corrections)

	// Constraints should appear before facts and preferences
	constraintIdx := strings.Index(out, "Never delete the queue.")
	factIdx := strings.Index(out, "A regular fact.")
	prefIdx := strings.Index(out, "Prefer prose.")

	if constraintIdx > factIdx {
		t.Error("constraint should render before fact")
	}
	if constraintIdx > prefIdx {
		t.Error("constraint should render before preference")
	}
}

func TestScopeClusteringTriggersAt3(t *testing.T) {
	// 3 corrections sharing a scope should be clustered
	corrections := []db.Correction{
		{Scope: "project:foo", Fact: "Fact one.", Type: "fact"},
		{Scope: "project:foo", Fact: "Fact two.", Type: "fact"},
		{Scope: "project:foo", Fact: "Fact three.", Type: "fact"},
		{Scope: "global", Fact: "Global fact.", Type: "fact"},
	}
	out := FormatMemoryBlock(corrections)

	// Should have a scope header for project:foo since 3 share it
	if !strings.Contains(out, "[project:foo]") {
		t.Errorf("expected scope cluster header for project:foo, got:\n%s", out)
	}
	// Global has only 1, so it should be inline with [global] prefix
	if !strings.Contains(out, "[global]") {
		t.Errorf("expected inline scope for global, got:\n%s", out)
	}
}

func TestScopeClusteringNoClusterAt2(t *testing.T) {
	// 2 corrections should NOT trigger clustering
	corrections := []db.Correction{
		{Scope: "project:foo", Fact: "Fact one.", Type: "fact"},
		{Scope: "project:foo", Fact: "Fact two.", Type: "fact"},
	}
	out := FormatMemoryBlock(corrections)

	// Both should have inline [project:foo] prefix
	count := strings.Count(out, "[project:foo]")
	if count != 2 {
		t.Errorf("expected 2 inline scope prefixes, got %d in:\n%s", count, out)
	}
}

func TestCompositeScoreHighHitCountWins(t *testing.T) {
	highHits := db.ScoredCorrection{
		Correction: db.Correction{HitCount: 50, Confidence: 1.0},
		Score:      -5.0,
	}
	noHits := db.ScoredCorrection{
		Correction: db.Correction{HitCount: 0, Confidence: 1.0},
		Score:      -5.0, // same BM25
	}
	if compositeScore(highHits, "") <= compositeScore(noHits, "") {
		t.Error("high hit_count should outrank zero-hit with same BM25")
	}
}

func TestCompositeScoreRecencyMatters(t *testing.T) {
	recent := db.ScoredCorrection{
		Correction: db.Correction{
			HitCount:   5,
			Confidence: 1.0,
			LastHit:    sql.NullInt64{Int64: time.Now().Unix(), Valid: true},
		},
		Score: -5.0,
	}
	old := db.ScoredCorrection{
		Correction: db.Correction{
			HitCount:   5,
			Confidence: 1.0,
			LastHit:    sql.NullInt64{Int64: time.Now().Unix() - 365*86400, Valid: true},
		},
		Score: -5.0,
	}
	if compositeScore(recent, "") <= compositeScore(old, "") {
		t.Error("recent correction should outrank stale equivalent")
	}
}

func TestSelectCorrectionsProjectPriority(t *testing.T) {
	all := []db.ScoredCorrection{
		{Correction: db.Correction{ID: 1, Fact: "alpha bravo charlie", Scope: "domain:go", Confidence: 1.0}, Score: -5.0},
		{Correction: db.Correction{ID: 2, Fact: "delta echo foxtrot", Scope: "global", Confidence: 1.0}, Score: -5.0},
		{Correction: db.Correction{ID: 3, Fact: "golf hotel india", Scope: "project:myproject", Confidence: 1.0}, Score: -5.0},
	}

	selected := SelectCorrections(all, 10, 3000, "myproject")
	if len(selected) != 3 {
		t.Fatalf("expected 3, got %d", len(selected))
	}
	// With tier bonus: project (1.5) > global (1.2) > domain (1.0)
	if selected[0].Scope != "project:myproject" {
		t.Errorf("expected project first, got %q", selected[0].Scope)
	}
	if selected[1].Scope != "global" {
		t.Errorf("expected global second, got %q", selected[1].Scope)
	}
	if selected[2].Scope != "domain:go" {
		t.Errorf("expected domain third, got %q", selected[2].Scope)
	}
}

func TestWrongFieldNeverInjected(t *testing.T) {
	corrections := []db.Correction{
		{
			Scope: "global",
			Fact:  "Use toml not viper.",
			Wrong: sql.NullString{String: "viper was wrong", Valid: true},
		},
	}
	out := FormatMemoryBlock(corrections)
	if strings.Contains(out, "viper was wrong") {
		t.Error("wrong field appeared in memory block output")
	}
}

// §4: compositeScore neutral signal test
func TestCompositeScoreNeutralVsStrong(t *testing.T) {
	strong := db.ScoredCorrection{
		Correction: db.Correction{Confidence: 1.0},
		Score:      -5.0, // strong BM25
	}
	neutral := db.ScoredCorrection{
		Correction: db.Correction{Confidence: 1.0},
		Score:      0, // no BM25 signal
	}
	if compositeScore(strong, "") <= compositeScore(neutral, "") {
		t.Error("strong BM25 should rank above neutral")
	}
}

func TestCompositeScoreNeutralRanksByHitCount(t *testing.T) {
	highHits := db.ScoredCorrection{
		Correction: db.Correction{HitCount: 20, Confidence: 1.0},
		Score:      0, // neutral
	}
	noHits := db.ScoredCorrection{
		Correction: db.Correction{HitCount: 0, Confidence: 1.0},
		Score:      0, // neutral
	}
	if compositeScore(highHits, "") <= compositeScore(noHits, "") {
		t.Error("higher hit_count should rank above zero hits when both neutral BM25")
	}
}

// §8: Recency floor test
func TestRecencyDecay365(t *testing.T) {
	recent := db.ScoredCorrection{
		Correction: db.Correction{
			Confidence: 1.0,
			LastHit:    sql.NullInt64{Int64: time.Now().Unix() - 30*86400, Valid: true}, // 30 days ago
		},
		Score: -5.0,
	}
	old := db.ScoredCorrection{
		Correction: db.Correction{
			Confidence: 1.0,
			LastHit:    sql.NullInt64{Int64: time.Now().Unix() - 3*365*86400, Valid: true}, // 3 years ago
		},
		Score: -5.0,
	}
	recentScore := compositeScore(recent, "")
	oldScore := compositeScore(old, "")
	if recentScore <= oldScore {
		t.Errorf("30-day-old should rank above 3-year-old: recent=%f old=%f", recentScore, oldScore)
	}
}

func TestRecencyNoLastHitFullWeight(t *testing.T) {
	noHit := db.ScoredCorrection{
		Correction: db.Correction{Confidence: 1.0},
		Score:      -5.0,
	}
	oldHit := db.ScoredCorrection{
		Correction: db.Correction{
			Confidence: 1.0,
			LastHit:    sql.NullInt64{Int64: time.Now().Unix() - 365*86400, Valid: true},
		},
		Score: -5.0,
	}
	// No last_hit should get full recency weight (1.0), which beats decayed
	if compositeScore(noHit, "") < compositeScore(oldHit, "") {
		t.Error("no-last-hit should get full recency weight, beating 1-year-old")
	}
}

// §9: Tier bonus tests
func TestTierBonusProjectBeatsGlobal(t *testing.T) {
	proj := db.ScoredCorrection{
		Correction: db.Correction{Fact: "project specific info here", Scope: "project:myproject", Confidence: 1.0},
		Score:      -5.0,
	}
	glob := db.ScoredCorrection{
		Correction: db.Correction{Fact: "global information here", Scope: "global", Confidence: 1.0},
		Score:      -5.0,
	}
	if compositeScore(proj, "myproject") <= compositeScore(glob, "myproject") {
		t.Error("project-scoped correction should beat global with same BM25 (1.5 vs 1.2)")
	}
}

func TestTierBonusStrongGlobalBeatsWeakProject(t *testing.T) {
	weakProj := db.ScoredCorrection{
		Correction: db.Correction{Fact: "vague project information", Scope: "project:myproject", Confidence: 1.0},
		Score:      -1.0, // weak BM25
	}
	strongGlob := db.ScoredCorrection{
		Correction: db.Correction{Fact: "strong global match data", Scope: "global", Confidence: 1.0},
		Score:      -20.0, // very strong BM25
	}
	if compositeScore(strongGlob, "myproject") <= compositeScore(weakProj, "myproject") {
		t.Error("strong global should beat weak project (soft preference, not hard tier)")
	}
}

func TestTierBonusDomainBaseline(t *testing.T) {
	domain := db.ScoredCorrection{
		Correction: db.Correction{Fact: "domain specific thing", Scope: "domain:go", Confidence: 1.0},
		Score:      -5.0,
	}
	score := compositeScore(domain, "myproject")
	// Domain gets tierBonus=1.0, compute expected
	glob := db.ScoredCorrection{
		Correction: db.Correction{Fact: "global specific thing", Scope: "global", Confidence: 1.0},
		Score:      -5.0,
	}
	globScore := compositeScore(glob, "myproject")
	// Global should be 1.2/1.0 = 20% higher
	ratio := globScore / score
	if ratio < 1.15 || ratio > 1.25 {
		t.Errorf("expected global/domain ratio ~1.2, got %.2f", ratio)
	}
}

// §10: MMR diversity tests
func TestMMRDiversityNearDuplicates(t *testing.T) {
	// All have equal BM25 scores so diversity is the tiebreaker
	all := []db.ScoredCorrection{
		{Correction: db.Correction{ID: 1, Fact: "project uses toml config parser library here", Scope: "global", Confidence: 1.0}, Score: -5.0},
		{Correction: db.Correction{ID: 2, Fact: "config files parsed using toml parser library", Scope: "global", Confidence: 1.0}, Score: -5.0},
		{Correction: db.Correction{ID: 3, Fact: "toml config parser chosen over viper library", Scope: "global", Confidence: 1.0}, Score: -5.0},
		{Correction: db.Correction{ID: 4, Fact: "toml format used for config parser data here", Scope: "global", Confidence: 1.0}, Score: -5.0},
		{Correction: db.Correction{ID: 5, Fact: "toml parser package config library from burntsushi", Scope: "global", Confidence: 1.0}, Score: -5.0},
		{Correction: db.Correction{ID: 6, Fact: "tests run with race detection for concurrency checking", Scope: "global", Confidence: 1.0}, Score: -5.0},
	}

	selected := SelectCorrections(all, 3, 3000, "")
	if len(selected) != 3 {
		t.Fatalf("expected 3, got %d", len(selected))
	}

	// The unrelated correction (test-race, ID 6) should appear within the top 3
	// because MMR penalizes the near-duplicate toml corrections
	foundUnrelated := false
	for _, s := range selected {
		if s.ID == 6 {
			foundUnrelated = true
		}
	}
	if !foundUnrelated {
		ids := make([]int64, len(selected))
		for i, s := range selected {
			ids[i] = s.ID
		}
		t.Errorf("MMR should include the diverse correction (ID 6); got IDs %v", ids)
	}
}

func TestMMRTokenBudgetSkip(t *testing.T) {
	all := []db.ScoredCorrection{
		{Correction: db.Correction{ID: 1, Fact: strings.Repeat("long ", 100), Scope: "global", Confidence: 1.0}, Score: -10.0},
		{Correction: db.Correction{ID: 2, Fact: "short fact", Scope: "global", Confidence: 1.0}, Score: -9.0},
	}

	// Budget too tight for the first (long) correction
	selected := SelectCorrections(all, 10, 50, "")
	if len(selected) != 1 {
		t.Fatalf("expected 1 (budget skip), got %d", len(selected))
	}
	if selected[0].ID != 2 {
		t.Error("should have skipped long correction and picked short one")
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		a, b     string
		expected float64
	}{
		{"", "", 0},
		{"hello world", "hello world", 1.0},
		{"cat dog mouse", "cat dog bird", 0.5},
	}
	for _, tt := range tests {
		got := JaccardSimilarity(tt.a, tt.b)
		if got < tt.expected-0.01 || got > tt.expected+0.01 {
			t.Errorf("JaccardSimilarity(%q, %q) = %.2f, want %.2f", tt.a, tt.b, got, tt.expected)
		}
	}
}
