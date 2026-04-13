package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type exportCorrection struct {
	ID         int64   `json:"id" toml:"id"`
	Fact       string  `json:"fact" toml:"fact"`
	Wrong      string  `json:"wrong,omitempty" toml:"wrong,omitempty"`
	Scope      string  `json:"scope" toml:"scope"`
	Tags       string  `json:"tags,omitempty" toml:"tags,omitempty"`
	Source     string  `json:"source,omitempty" toml:"source,omitempty"`
	Confidence float64 `json:"confidence" toml:"confidence"`
	CreatedAt  int64   `json:"created_at" toml:"created_at"`
	UpdatedAt  int64   `json:"updated_at" toml:"updated_at"`
	HitCount   int64   `json:"hit_count" toml:"hit_count"`
}

type exportData struct {
	Corrections []exportCorrection `json:"corrections" toml:"corrections"`
}

func Export(args []string, dbPath string) error {
	var format, output string

	a := newArgs(args, "Usage: engram export [flags]")
	a.String(&format, "format", "json", "Output format: json or toml")
	a.String(&output, "output", "", "Output file (default: stdout)")
	a.String(&output, "o", "", "Output file (short)")
	if err := a.Parse(); err != nil {
		return err
	}

	database, _, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	corrections, err := database.List("", "", 0)
	if err != nil {
		return err
	}

	var export exportData
	for _, c := range corrections {
		export.Corrections = append(export.Corrections, exportCorrection{
			ID:         c.ID,
			Fact:       c.Fact,
			Wrong:      c.Wrong.String,
			Scope:      c.Scope,
			Tags:       c.Tags.String,
			Source:     c.Source.String,
			Confidence: c.Confidence,
			CreatedAt:  c.CreatedAt,
			UpdatedAt:  c.UpdatedAt,
			HitCount:   c.HitCount,
		})
	}

	w := os.Stdout
	if output != "" {
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	switch format {
	case "toml":
		return toml.NewEncoder(w).Encode(export)
	default:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(export)
	}
}
