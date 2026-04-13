package cmd

import (
	"fmt"
	"time"
)

func List(args []string, dbPath string) error {
	var scope, tag string
	var limit int

	a := newArgs(args, "Usage: engram list [flags]")
	a.String(&scope, "scope", "", "Filter by scope (e.g. global, project:foo)")
	a.String(&tag, "tag", "", "Filter by tag")
	a.Int(&limit, "limit", 50, "Maximum results")
	if err := a.Parse(); err != nil {
		return err
	}

	database, _, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	corrections, err := database.List(scope, tag, limit)
	if err != nil {
		return err
	}

	if len(corrections) == 0 {
		fmt.Println("No corrections stored.")
		return nil
	}

	for _, c := range corrections {
		ts := time.Unix(c.CreatedAt, 0).Format("2006-01-02")
		fmt.Printf("#%-4d [%s] %s  (%s)\n", c.ID, c.Scope, c.Fact, ts)
	}
	fmt.Printf("\n%d correction(s)\n", len(corrections))
	return nil
}
