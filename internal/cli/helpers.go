package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/runnerr0/chronicle/internal/storage"
)

const defaultRetentionDays = 30

// defaultDBPath returns the default Chronicle database path.
func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "chronicle.db"
	}
	return filepath.Join(home, ".config", "fabric", "chronicle", "chronicle.db")
}

// openDefaultStore opens the default Chronicle database, runs migrations,
// and returns a ready-to-use store and the underlying *sql.DB.
func openDefaultStore() (*storage.SQLiteStore, *sql.DB, error) {
	dbPath := defaultDBPath()

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	runner := storage.NewMigrationRunner(db)
	if err := runner.Run(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	store, err := storage.NewSQLiteStore(db)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create store: %w", err)
	}

	return store, db, nil
}

// parseDuration parses a human-friendly duration string like "30d", "7d", "24h", "2w".
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid duration: empty string")
	}

	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %q", s)
	}

	suffix := s[len(s)-1]
	numStr := s[:len(s)-1]

	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration: %q", s)
	}

	switch suffix {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case 'm':
		return time.Duration(n) * time.Minute, nil
	default:
		return 0, fmt.Errorf("invalid duration: %q (use d, h, w, or m suffix)", s)
	}
}

// formatDurationHuman formats a duration into a human-readable string like "30 days".
func formatDurationHuman(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days > 0 {
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	hours := int(d.Hours())
	if hours > 0 {
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	return d.String()
}
