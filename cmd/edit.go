package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"engram/internal/db"
)

func Edit(args []string, dbPath string) error {
	a := newArgs(args, "Usage: engram edit <id>")
	if err := a.Parse(); err != nil {
		return err
	}

	positional := a.Positional()
	if len(positional) != 1 {
		return fmt.Errorf("expected exactly one ID\n\nUsage: engram edit <id>")
	}

	id, err := strconv.ParseInt(positional[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid ID %q: expected a number", positional[0])
	}
	if id <= 0 {
		return fmt.Errorf("ID must be a positive integer")
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	database, _, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	c, err := database.Get(id)
	if err != nil {
		return err
	}

	type editableFields struct {
		Fact        string `json:"fact"`
		Scope       string `json:"scope"`
		Tags        string `json:"tags"`
		Type        string `json:"type"`
		TriggerHint string `json:"trigger_hint"`
	}
	ef := editableFields{
		Fact:        c.Fact,
		Scope:       c.Scope,
		Tags:        c.Tags.String,
		Type:        c.Type,
		TriggerHint: c.TriggerHint.String,
	}

	tmpFile, err := os.CreateTemp("", "engram-edit-*.json")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	enc := json.NewEncoder(tmpFile)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ef); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()

	editorCmd := exec.Command(editor, tmpPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("reading edited file: %w", err)
	}

	var updated editableFields
	if err := json.Unmarshal(data, &updated); err != nil {
		return fmt.Errorf("parsing edited file (invalid JSON): %w", err)
	}

	if updated.Fact == "" {
		return fmt.Errorf("fact cannot be empty")
	}

	fields := db.UpdateFields{
		Fact:        &updated.Fact,
		Scope:       &updated.Scope,
		Tags:        &updated.Tags,
		Type:        &updated.Type,
		TriggerHint: &updated.TriggerHint,
	}
	if err := database.Update(id, fields); err != nil {
		return err
	}

	fmt.Printf("Updated correction #%d\n", id)
	return nil
}
