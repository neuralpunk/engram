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

	maxCorrections := cfg.Injection.MaxCorrections
	if limit > 0 {
		maxCorrections = limit
	}

	detectedProject := ""
	detectedScope := ""
	if projName, found := project.Detect(workdir); found {
		detectedProject = projName
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
		var corrections []db.Correction
		var err error
		if len(scopes) > 0 {
			corrections, err = database.ListByScopes(scopes, "", 0)
		} else {
			corrections, err = database.List("", "", 0)
		}
		if err != nil {
			database.Close()
			return err
		}

		scored := make([]db.ScoredCorrection, len(corrections))
		for i, c := range corrections {
			// 0 = no BM25 signal; compositeScore treats this as neutral
			scored[i] = db.ScoredCorrection{Correction: c, Score: 0}
		}
		selected = format.SelectCorrections(scored, maxCorrections, cfg.Injection.MaxTokens, detectedProject)
	} else {
		results, err := database.Search(query, scopes, maxCorrections*2, cfg.Injection.MinScore)
		if err != nil {
			database.Close()
			return err
		}
		selected = format.SelectCorrections(results, maxCorrections, cfg.Injection.MaxTokens, detectedProject)
	}

	// Print output first, then update hit counts async
	if raw {
		for _, c := range selected {
			fmt.Printf("[%s] %s\n", c.Scope, c.Fact)
		}
		if len(selected) == 0 {
			database.Close()
			return nil
		}
	} else {
		fmt.Print(format.FormatSystemPrompt(selected))
	}

	// Update hit counts before closing
	ids := make([]int64, len(selected))
	for i, c := range selected {
		ids[i] = c.ID
	}
	database.IncrementHitCounts(ids)
	database.Close()

	return nil
}
