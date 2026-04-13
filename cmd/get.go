package cmd

import (
	"fmt"
	"strings"

	"engram/internal/db"
	"engram/internal/format"
	"engram/internal/project"
)

func Get(args []string, dbPath string) error {
	var scope, workdir string
	var limit int
	var raw, all bool

	a := newArgs(args, "Usage: engram get [query] [flags]")
	a.String(&scope, "scope", "all", "Scope filter: global, project:<name>, domain:<tag>, or all")
	a.Int(&limit, "limit", 0, "Maximum corrections (default: from config)")
	a.Bool(&raw, "raw", false, "Output plain text instead of XML block")
	a.Bool(&all, "all", false, "Return all corrections for current scope")
	a.String(&workdir, "workdir", ".", "Working directory for project detection")
	if err := a.Parse(); err != nil {
		return err
	}

	query := strings.Join(a.Positional(), " ")

	database, cfg, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	maxCorrections := cfg.Injection.MaxCorrections
	if limit > 0 {
		maxCorrections = limit
	}

	detectedScope := ""
	if projName, found := project.Detect(workdir); found {
		detectedScope = "project:" + projName
	}

	var scopes []string
	if scope != "" && scope != "all" {
		scopes = []string{scope}
	} else if detectedScope != "" {
		scopes = []string{"global", detectedScope}
	}

	var selected []db.Correction

	if all || query == "" {
		corrections, err := database.List("", "", 0)
		if err != nil {
			return err
		}

		if len(scopes) > 0 {
			scopeSet := make(map[string]bool, len(scopes))
			for _, s := range scopes {
				scopeSet[s] = true
			}
			filtered := corrections[:0:0]
			for _, c := range corrections {
				if scopeSet[c.Scope] {
					filtered = append(filtered, c)
				}
			}
			corrections = filtered
		}

		scored := make([]db.ScoredCorrection, len(corrections))
		for i, c := range corrections {
			scored[i] = db.ScoredCorrection{Correction: c, Score: -1.0}
		}
		selected = format.SelectCorrections(scored, maxCorrections, cfg.Injection.MaxTokens)
	} else {
		results, err := database.Search(query, scopes, maxCorrections*2, cfg.Injection.MinScore)
		if err != nil {
			return err
		}
		selected = format.SelectCorrections(results, maxCorrections, cfg.Injection.MaxTokens)
	}

	if len(selected) == 0 {
		return nil
	}

	ids := make([]int64, len(selected))
	for i, c := range selected {
		ids[i] = c.ID
	}
	database.IncrementHitCounts(ids)

	if raw {
		for _, c := range selected {
			fmt.Printf("[%s] %s\n", c.Scope, c.Fact)
		}
	} else {
		fmt.Print(format.FormatMemoryBlock(selected))
	}

	return nil
}
