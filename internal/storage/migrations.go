package storage

import (
	"database/sql"
	"fmt"
)

// migration represents a single schema migration.
type migration struct {
	Version int
	Name    string
	Apply   func(tx *sql.Tx) error
}

// MigrationRunner applies pending migrations to a SQLite database.
type MigrationRunner struct {
	db         *sql.DB
	migrations []migration
}

// NewMigrationRunner creates a MigrationRunner with all registered migrations.
func NewMigrationRunner(db *sql.DB) *MigrationRunner {
	return &MigrationRunner{
		db: db,
		migrations: []migration{
			{Version: 1, Name: "initial_schema", Apply: migrateV001},
		},
	}
}

// Run applies all pending migrations in order. It enables WAL mode and
// foreign keys, creates the schema_migrations tracking table, then applies
// each migration that hasn't been recorded yet.
func (r *MigrationRunner) Run() error {
	// Enable WAL mode for concurrent read performance.
	if _, err := r.db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return fmt.Errorf("set WAL mode: %w", err)
	}

	// Enable foreign key enforcement.
	if _, err := r.db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}

	// Ensure the schema_migrations table exists.
	if _, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			name       TEXT NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	for _, m := range r.migrations {
		applied, err := r.isApplied(m.Version)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", m.Version, err)
		}
		if applied {
			continue
		}

		if err := r.apply(m); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", m.Version, m.Name, err)
		}
	}

	return nil
}

// isApplied checks whether a migration version has already been recorded.
func (r *MigrationRunner) isApplied(version int) (bool, error) {
	var count int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// apply executes a migration inside a transaction and records it.
func (r *MigrationRunner) apply(m migration) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := m.Apply(tx); err != nil {
		return err
	}

	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
		m.Version, m.Name,
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}
