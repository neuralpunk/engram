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

// dsn encodes pragmas that can be set at connection time.
// mmap_size and cache_size are set dynamically after open.
const dsn = "%s?_journal_mode=WAL&_synchronous=NORMAL&_foreign_keys=on&_temp_store=MEMORY"

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

	conn, err := sql.Open("sqlite3", fmt.Sprintf(dsn, path))
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// SQLite: one writer at a time, keep connection warm
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)

	// Restrict database file permissions (user read/write only)
	if err := os.Chmod(path, 0600); err != nil && !os.IsNotExist(err) {
		// Ignore if file doesn't exist yet (will be created on first write)
	}

	db := &DB{conn: conn}
	db.setAdaptiveMmap(path)
	db.conn.Exec("PRAGMA cache_size=-65536") // 64MB page cache
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

// OpenMemory opens an in-memory SQLite database for testing.
func OpenMemory() (*DB, error) {
	conn, err := sql.Open("sqlite3", ":memory:?_journal_mode=WAL&_synchronous=NORMAL&_foreign_keys=on&_temp_store=MEMORY")
	if err != nil {
		return nil, err
	}

	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)

	db := &DB{conn: conn}
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

// setAdaptiveMmap sets mmap_size based on actual DB file size.
// Maps at least 4x the current file to handle growth, capped at 512MB.
func (db *DB) setAdaptiveMmap(path string) {
	info, err := os.Stat(path)
	if err != nil {
		db.conn.Exec("PRAGMA mmap_size=134217728") // fallback 128MB
		return
	}

	size := info.Size() * 4
	const cap = 512 * 1024 * 1024 // 512MB
	if size > cap {
		size = cap
	}
	if size < 32*1024*1024 { // floor 32MB
		size = 32 * 1024 * 1024
	}
	db.conn.Exec(fmt.Sprintf("PRAGMA mmap_size=%d", size))
}

// Vacuum runs incremental vacuum and PRAGMA optimize.
func (db *DB) Vacuum() error {
	if _, err := db.conn.Exec("PRAGMA incremental_vacuum"); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	_, err := db.conn.Exec("PRAGMA optimize")
	return err
}

// RebuildFTS rebuilds the FTS5 index from scratch.
func (db *DB) RebuildFTS() error {
	_, err := db.conn.Exec("INSERT INTO corrections_fts(corrections_fts) VALUES('rebuild')")
	return err
}

func (db *DB) migrate() error {
	var version int
	err := db.conn.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_version",
	).Scan(&version)
	if err != nil {
		version = 0 // schema_version table doesn't exist yet
	}
	if version >= 1 {
		return nil // already applied
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
