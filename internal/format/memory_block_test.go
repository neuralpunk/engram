package format

import (
	"database/sql"
	"strings"
	"testing"

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
		{Scope: "global", Fact: "Prefer prose over bullet points."},
		{Scope: "project:foo", Fact: "Uses BurntSushi/toml."},
	}
	out := FormatMemoryBlock(corrections)

	if !strings.Contains(out, "<engram-memory>") {
		t.Error("missing opening tag")
	}
	if !strings.Contains(out, "</engram-memory>") {
		t.Error("missing closing tag")
	}
	if !strings.Contains(out, "1. [global] Prefer prose over bullet points.") {
		t.Errorf("missing first correction in output: %s", out)
	}
	if !strings.Contains(out, "2. [project:foo] Uses BurntSushi/toml.") {
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
	if !strings.Contains(out, "store_correction") {
		t.Error("missing tool reference in behavior prompt")
	}
}

func TestSelectCorrectionsGlobalsFirst(t *testing.T) {
	all := []db.ScoredCorrection{
		{Correction: db.Correction{ID: 1, Fact: "project fact", Scope: "project:foo"}, Score: -5.0},
		{Correction: db.Correction{ID: 2, Fact: "global fact", Scope: "global"}, Score: -3.0},
		{Correction: db.Correction{ID: 3, Fact: "another project", Scope: "project:bar"}, Score: -4.0},
	}

	selected := SelectCorrections(all, 10, 300)
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

	selected := SelectCorrections(all, 2, 3000)
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

	selected := SelectCorrections(all, 10, 50) // tight budget
	if len(selected) != 1 {
		t.Errorf("expected 1 within budget, got %d", len(selected))
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
