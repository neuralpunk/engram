package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAtRoot(t *testing.T) {
	dir := t.TempDir()
	engramContent := `project = "myproject"` + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".engram"), []byte(engramContent), 0644); err != nil {
		t.Fatal(err)
	}

	name, found := Detect(dir)
	if !found {
		t.Fatal("expected to find .engram")
	}
	if name != "myproject" {
		t.Errorf("expected myproject, got %s", name)
	}
}

func TestDetectWalksUp(t *testing.T) {
	root := t.TempDir()
	engramContent := `project = "deepproject"` + "\n"
	if err := os.WriteFile(filepath.Join(root, ".engram"), []byte(engramContent), 0644); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	name, found := Detect(nested)
	if !found {
		t.Fatal("expected to find .engram walking up")
	}
	if name != "deepproject" {
		t.Errorf("expected deepproject, got %s", name)
	}
}

func TestDetectNotFound(t *testing.T) {
	dir := t.TempDir()
	_, found := Detect(dir)
	if found {
		t.Error("expected no .engram found in temp dir")
	}
}

func TestDetectEmptyProject(t *testing.T) {
	dir := t.TempDir()
	// .engram file exists but has no project field
	if err := os.WriteFile(filepath.Join(dir, ".engram"), []byte("# comment only\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, found := Detect(dir)
	if found {
		t.Error("expected not found when project field is empty")
	}
}
