package cmd

import "fmt"

func Vacuum(args []string, dbPath string) error {
	a := newArgs(args, "Usage: engram vacuum")
	if err := a.Parse(); err != nil {
		return err
	}

	database, _, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	fmt.Print("Running incremental vacuum... ")
	if err := database.Vacuum(); err != nil {
		return err
	}
	fmt.Println("done")

	fmt.Print("Rebuilding FTS5 index... ")
	if err := database.RebuildFTS(); err != nil {
		return err
	}
	fmt.Println("done")

	return nil
}
