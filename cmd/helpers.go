package cmd

import (
	"engram/internal/config"
	"engram/internal/db"
)

// openDB opens the database, using dbPath directly if provided,
// otherwise loading the path from config.
func openDB(dbPath string) (*db.DB, *config.Config, error) {
	if dbPath != "" {
		cfg := config.DefaultConfig()
		database, err := db.Open(dbPath)
		if err != nil {
			return nil, nil, err
		}
		return database, &cfg, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	database, err := db.Open(cfg.ResolveDatabasePath())
	if err != nil {
		return nil, nil, err
	}
	return database, &cfg, nil
}
