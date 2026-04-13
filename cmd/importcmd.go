package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"engram/internal/db"
)

func Import(args []string, dbPath string) error {
	var format string

	a := newArgs(args, "Usage: engram import <file> [flags]")
	a.String(&format, "format", "", "Input format: json or toml (default: detect from extension)")
	if err := a.Parse(); err != nil {
		return err
	}

	positional := a.Positional()
	if len(positional) != 1 {
		return fmt.Errorf("expected exactly one file path\n\nUsage: engram import <file> [--format json|toml]")
	}

	filePath := positional[0]

	database, _, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	if format == "" {
		switch strings.ToLower(filepath.Ext(filePath)) {
		case ".toml":
			format = "toml"
		default:
			format = "json"
		}
	}

	var imported exportData
	switch format {
	case "toml":
		if err := toml.Unmarshal(data, &imported); err != nil {
			return fmt.Errorf("parsing TOML: %w", err)
		}
	default:
		if err := json.Unmarshal(data, &imported); err != nil {
			return fmt.Errorf("parsing JSON: %w", err)
		}
	}

	count := 0
	for _, ec := range imported.Corrections {
		c := &db.Correction{
			Fact:       ec.Fact,
			Wrong:      sql.NullString{String: ec.Wrong, Valid: ec.Wrong != ""},
			Scope:      ec.Scope,
			Tags:       sql.NullString{String: ec.Tags, Valid: ec.Tags != ""},
			Source:     sql.NullString{String: ec.Source, Valid: ec.Source != ""},
			Confidence: ec.Confidence,
		}
		if c.Scope == "" {
			c.Scope = "global"
		}
		if c.Confidence == 0 {
			c.Confidence = 1.0
		}
		if _, err := database.Store(c); err != nil {
			return fmt.Errorf("importing correction %q: %w", ec.Fact, err)
		}
		count++
	}

	fmt.Printf("Imported %d correction(s)\n", count)
	return nil
}
