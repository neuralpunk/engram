package cmd

import "fmt"

func Stats(args []string, dbPath string) error {
	a := newArgs(args, "Usage: engram stats")
	if err := a.Parse(); err != nil {
		return err
	}

	database, _, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	stats, err := database.Stats()
	if err != nil {
		return err
	}

	fmt.Printf("Corrections:  %d\n", stats.TotalCorrections)
	fmt.Printf("Sessions:     %d\n", stats.TotalSessions)
	fmt.Printf("Injections:   %d\n", stats.TotalInjections)

	if len(stats.TypeBreakdown) > 0 {
		fmt.Printf("\nBy type:\n")
		typeOrder := []string{"fact", "constraint", "preference", "gotcha", "process"}
		for _, t := range typeOrder {
			if count, ok := stats.TypeBreakdown[t]; ok {
				fmt.Printf("  %-14s %d\n", t, count)
			}
		}
	}

	if stats.StaleCount > 0 {
		fmt.Printf("\nStale (not retrieved in 180 days): %d\n", stats.StaleCount)
		fmt.Println("  Run 'engram list --stale' to review them.")
	}

	if len(stats.TopCorrections) > 0 {
		fmt.Printf("\nTop corrections by hit count:\n")
		for _, c := range stats.TopCorrections {
			fmt.Printf("  #%-4d [%s] hits:%-3d %s\n", c.ID, c.Scope, c.HitCount, c.Fact)
		}
	}
	return nil
}
