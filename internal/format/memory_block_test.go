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

func TestSelectCorrectionsGlobalsFirst(t *testing.T) {
	all := []db.ScoredCorrection{
		{Correction: db.Correction{ID: 1, Fact: "project fact", Scope: "project:foo"}, Score: -5.0},
		{Correction: db.Correction{ID: 2, Fact: "global fact", Scope: "global"}, Score: -3.0},
		{Correction: db.Correction{ID: 3, Fact: "another project", Scope: "project:bar"}, Score: -4.0},
	}

	selected := SelectCorrections(all, 10, 300, "")
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected, got %d", len(selected))
	}
	// Global should come first
	if selected[0].Scope != "global" {
		t.Errorf("expected global first, got %q", selected[0].Scope)
	}
}

func TestSelectCorrectionsMaxCount(t *testing.T) {
	all := []db.ScoredCorrection{
		{Correction: db.Correction{ID: 1, Fact: "fact 1", Scope: "global"}, Score: -5.0},
		{Correction: db.Correction{ID: 2, Fact: "fact 2", Scope: "global"}, Score: -3.0},
		{Correction: db.Correction{ID: 3, Fact: "fact 3", Scope: "global"}, Score: -4.0},
	}

	selected := SelectCorrections(all, 2, 3000, "")
	if len(selected) != 2 {
		t.Errorf("expected 2 selected, got %d", len(selected))
	}
}

func TestSelectCorrectionsTokenBudget(t *testing.T) {
	// Each entry will be roughly "[global] short" = 15 chars = 3 tokens
	all := []db.ScoredCorrection{
		{Correction: db.Correction{ID: 1, Fact: "short", Scope: "global"}, Score: -5.0},
		{Correction: db.Correction{ID: 2, Fact: strings.Repeat("x", 1200), Scope: "global"}, Score: -3.0}, // ~300 tokens
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
	if compositeScore(highHits) <= compositeScore(noHits) {
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
	if compositeScore(recent) <= compositeScore(old) {
		t.Error("recent correction should outrank stale equivalent")
	}
}

func TestSelectCorrectionsProjectPriority(t *testing.T) {
	all := []db.ScoredCorrection{
		{Correction: db.Correction{ID: 1, Fact: "domain fact", Scope: "domain:go"}, Score: -5.0},
		{Correction: db.Correction{ID: 2, Fact: "global fact", Scope: "global"}, Score: -5.0},
		{Correction: db.Correction{ID: 3, Fact: "project fact", Scope: "project:myproject"}, Score: -5.0},
	}

	selected := SelectCorrections(all, 10, 3000, "myproject")
	if len(selected) != 3 {
		t.Fatalf("expected 3, got %d", len(selected))
	}
	// Order should be: global, project:myproject, domain:go
	if selected[0].Scope != "global" {
		t.Errorf("expected global first, got %q", selected[0].Scope)
	}
	if selected[1].Scope != "project:myproject" {
		t.Errorf("expected project second, got %q", selected[1].Scope)
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
