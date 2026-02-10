// Package store provides persistent storage backed by SQLite.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver

	"github.com/soyeahso/hunter3/internal/logging"
)

// DB wraps a SQLite database connection with migration support.
type DB struct {
	sql *sql.DB
	log *logging.Logger
}

// Open opens (or creates) a SQLite database at the given path and runs migrations.
// Use ":memory:" for an in-memory database (useful for tests).
func Open(path string, log *logging.Logger) (*DB, error) {
	if path != ":memory:" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("creating db directory: %w", err)
		}
	}

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}

	// WAL mode for better concurrent read performance
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	// Foreign keys on
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
	}

	db := &DB{sql: sqlDB, log: log.Sub("store")}

	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	db.log.Info().Str("path", path).Msg("database opened")
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	db.log.Info().Msg("closing database")
	return db.sql.Close()
}

// SQL returns the underlying *sql.DB for direct queries.
func (db *DB) SQL() *sql.DB {
	return db.sql
}

// migrate runs all pending migrations.
func (db *DB) migrate() error {
	// Create migrations tracking table
	if _, err := db.sql.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	for _, m := range migrations {
		applied, err := db.isMigrationApplied(m.Version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		db.log.Info().Int("version", m.Version).Str("name", m.Name).Msg("applying migration")

		tx, err := db.sql.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.Version, err)
		}

		if _, err := tx.Exec(m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.Version); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.Version, err)
		}
	}

	return nil
}

func (db *DB) isMigrationApplied(version int) (bool, error) {
	var count int
	err := db.sql.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking migration %d: %w", version, err)
	}
	return count > 0, nil
}
