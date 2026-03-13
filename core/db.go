package core

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	conn *sql.DB
}

// Open opens the SQLite database at the given path and runs any pending
// migrations. The caller must call Close when done.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// Foreign keys are off by default in SQLite.
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	if _, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema_version table: %w", err)
	}

	var current int
	row := db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for i, entry := range entries {
		version := i + 1
		if version <= current {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for migration %d: %w", version, err)
		}
		statements := splitStatements(string(data))
		for _, stmt := range statements {
			if _, err := tx.Exec(stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("exec migration %d: %w", version, err)
			}
		}
		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("update schema version to %d: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", version, err)
		}
	}
	return nil
}

// splitStatements splits SQL text on semicolons, filtering empty statements.
func splitStatements(sql string) []string {
	parts := strings.Split(sql, ";")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
