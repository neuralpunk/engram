package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"engram/internal/db"
	"engram/internal/format"
	"engram/internal/project"
)

var validTypes = map[string]bool{
	"fact": true, "preference": true, "constraint": true,
	"gotcha": true, "process": true,
}

func Store(args []string, dbPath string) error {
	var wrong, scope, tags, source, workdir string
	var corrType, triggerHint string
	var supersedesID int64
	var force bool

	a := newArgs(args, "Usage: engram store <fact> [flags]")
	a.String(&scope, "scope", "", "Scope: global, project:<name>, domain:<tag> (auto-detect if omitted)")
	a.String(&wrong, "wrong", "", "What was previously assumed incorrectly")
	a.String(&tags, "tags", "", "Comma-separated tags (include synonyms and related concepts)")
	a.String(&source, "source", "user", "Source: user or inferred")
	a.String(&workdir, "workdir", ".", "Working directory for project detection")
	a.String(&corrType, "type", "fact", "Type: fact, preference, constraint, gotcha, process")
	a.String(&triggerHint, "trigger", "", "When to surface this correction (situation description)")
	a.Int64(&supersedesID, "supersedes", 0, "ID of the correction this replaces")
	a.Bool(&force, "force", false, "Bypass duplicate check and store anyway")
	if err := a.Parse(); err != nil {
		return err
	}

	positional := a.Positional()
	if len(positional) == 0 {
		return fmt.Errorf("missing fact\n\nUsage: engram store <fact> [--scope ...] [--tags ...] [--wrong ...]")
	}

	fact := strings.Join(positional, " ")

	if !validTypes[corrType] {
		return fmt.Errorf("invalid type %q: must be one of fact, preference, constraint, gotcha, process", corrType)
	}

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

	// Dedup check: look for similar corrections in the same scope
	if !force {
		results, err := database.Search(fact, []string{scope}, 5, 0)
		if err == nil {
			for _, r := range results {
				sim := format.JaccardSimilarity(fact, r.Fact)
				if sim >= 0.6 {
					fmt.Fprintf(os.Stderr, "Note: similar correction already exists (#%d): %q\n", r.ID, r.Fact)
					fmt.Fprintf(os.Stderr, "Use --supersedes %d to replace it, --force to store anyway, or do nothing.\n", r.ID)
					return nil
				}
			}
		}
	}

	c := &db.Correction{
		Fact:         fact,
		Wrong:        sql.NullString{String: wrong, Valid: wrong != ""},
		Scope:        scope,
		Tags:         sql.NullString{String: tags, Valid: tags != ""},
		Source:       sql.NullString{String: source, Valid: true},
		Type:         corrType,
		TriggerHint:  sql.NullString{String: triggerHint, Valid: triggerHint != ""},
		SupersedesID: sql.NullInt64{Int64: supersedesID, Valid: supersedesID > 0},
		Confidence:   confidence,
	}

	id, err := database.Store(c)
	if err != nil {
		return err
	}

	fmt.Printf("Stored correction #%d [%s]\n", id, scope)
	return nil
}
