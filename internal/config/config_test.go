package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Injection.MaxCorrections != 10 {
		t.Errorf("expected max_corrections 10, got %d", cfg.Injection.MaxCorrections)
	}
	if cfg.Injection.MaxTokens != 300 {
		t.Errorf("expected max_tokens 300, got %d", cfg.Injection.MaxTokens)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("ENGRAM_CONFIG", filepath.Join(t.TempDir(), "nonexistent.toml"))
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error for missing config, got %v", err)
	}
	if cfg.Injection.MaxCorrections != 10 {
		t.Errorf("expected default max_corrections, got %d", cfg.Injection.MaxCorrections)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
[database]
path = "/tmp/test.db"

[injection]
max_corrections = 20
max_tokens = 500
min_score = 0.5
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENGRAM_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database.Path != "/tmp/test.db" {
		t.Errorf("expected /tmp/test.db, got %s", cfg.Database.Path)
	}
	if cfg.Injection.MaxCorrections != 20 {
		t.Errorf("expected 20, got %d", cfg.Injection.MaxCorrections)
	}
	if cfg.Injection.MinScore != 0.5 {
		t.Errorf("expected 0.5, got %f", cfg.Injection.MinScore)
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"/absolute/path", "/absolute/path"},
		{"~/foo/bar", filepath.Join(home, "foo", "bar")},
		{"~/.config/engram", filepath.Join(home, ".config", "engram")},
	}

	for _, tt := range tests {
		got := expandHome(tt.input)
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
