package cmd

import (
	"fmt"
	"strconv"
)

func Delete(args []string, dbPath string) error {
	a := newArgs(args, "Usage: engram delete <id>")
	if err := a.Parse(); err != nil {
		return err
	}

	positional := a.Positional()
	if len(positional) != 1 {
		return fmt.Errorf("expected exactly one ID\n\nUsage: engram delete <id>")
	}

	id, err := strconv.ParseInt(positional[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid ID %q: expected a number", positional[0])
	}
	if id <= 0 {
		return fmt.Errorf("ID must be a positive integer")
	}

	database, _, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := database.Delete(id); err != nil {
		return err
	}
	fmt.Printf("Deleted correction #%d\n", id)
	return nil
}
