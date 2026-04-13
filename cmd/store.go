package cmd

import (
	"database/sql"
	"fmt"
	"strings"

	"engram/internal/db"
	"engram/internal/project"
)

func Store(args []string, dbPath string) error {
	var wrong, scope, tags, source, workdir string

	a := newArgs(args, "Usage: engram store <fact> [flags]")
	a.String(&scope, "scope", "", "Scope: global, project:<name>, domain:<tag> (auto-detect if omitted)")
	a.String(&wrong, "wrong", "", "What was previously assumed incorrectly")
	a.String(&tags, "tags", "", "Comma-separated tags (include synonyms and related concepts)")
	a.String(&source, "source", "user", "Source: user or inferred")
	a.String(&workdir, "workdir", ".", "Working directory for project detection")
	if err := a.Parse(); err != nil {
		return err
	}

	positional := a.Positional()
	if len(positional) == 0 {
		return fmt.Errorf("missing fact\n\nUsage: engram store <fact> [--scope ...] [--tags ...] [--wrong ...]")
	}

	fact := strings.Join(positional, " ")

	database, _, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	if scope == "" {
		if projName, found := project.Detect(workdir); found {
			scope = "project:" + projName
		} else {
			scope = "global"
		}
	}

	confidence := 1.0
	if source == "inferred" {
		confidence = 0.7
	}

	c := &db.Correction{
		Fact:       fact,
		Wrong:      sql.NullString{String: wrong, Valid: wrong != ""},
		Scope:      scope,
		Tags:       sql.NullString{String: tags, Valid: tags != ""},
		Source:     sql.NullString{String: source, Valid: true},
		Confidence: confidence,
	}

	id, err := database.Store(c)
	if err != nil {
		return err
	}

	fmt.Printf("Stored correction #%d [%s]\n", id, scope)
	return nil
}
