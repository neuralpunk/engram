package cmd

import (
	"fmt"
	"strings"
)

func Search(args []string, dbPath string) error {
	var limit int

	a := newArgs(args, "Usage: engram search <query> [flags]")
	a.Int(&limit, "limit", 10, "Maximum results")
	if err := a.Parse(); err != nil {
		return err
	}

	positional := a.Positional()
	if len(positional) == 0 {
		return fmt.Errorf("missing query\n\nUsage: engram search <query> [--limit N]")
	}

	query := strings.Join(positional, " ")

	database, _, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	results, err := database.Search(query, nil, limit, 0)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Println("No matching corrections.")
		return nil
	}

	for _, r := range results {
		fmt.Printf("#%-4d [%s] (score: %.2f) %s\n", r.ID, r.Scope, -r.Score, r.Fact)
	}
	return nil
}
