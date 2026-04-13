package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema/*.sql
var schemaFS embed.FS

// DB wraps a sql.DB connection to the engram database.
type DB struct {
	conn *sql.DB
}

// Open opens or creates the SQLite database at path and runs migrations.
// The database file and directory are created with restrictive permissions.
func Open(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Restrict database file permissions (user read/write only)
	if err := os.Chmod(path, 0600); err != nil && !os.IsNotExist(err) {
		// Ignore if file doesn't exist yet (will be created on first write)
	}

	db := &DB{conn: conn}
	if err := db.setPragmas(); err != nil {
		conn.Close()
		return nil, err
	}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

// OpenMemory opens an in-memory SQLite database for testing.
func OpenMemory() (*DB, error) {
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.setPragmas(); err != nil {
		conn.Close()
		return nil, err
	}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying sql.DB for advanced usage.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) setPragmas() error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA mmap_size=134217728",
	}
	for _, p := range pragmas {
		if _, err := db.conn.Exec(p); err != nil {
			return fmt.Errorf("setting pragma %q: %w", p, err)
		}
	}
	return nil
}

func (db *DB) migrate() error {
	// Check current schema version
	var version int
	err := db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	if err != nil {
		// Table doesn't exist yet, version is 0
		version = 0
	}

	if version >= 1 {
		return nil // already migrated
	}

	schema, err := schemaFS.ReadFile("schema/001_initial.sql")
	if err != nil {
		return fmt.Errorf("reading schema: %w", err)
	}

	if _, err := db.conn.Exec(string(schema)); err != nil {
		return fmt.Errorf("applying schema: %w", err)
	}
	return nil
}
