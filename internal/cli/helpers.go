package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/runnerr0/chronicle/internal/config"
	"github.com/runnerr0/chronicle/internal/storage"
)

const defaultRetentionDays = 30

// defaultDBPath returns the default Chronicle database path using the config system.
func defaultDBPath() string {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	storagePath := cfg.Storage.Path
	if strings.HasPrefix(storagePath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "chronicle.db"
		}
		storagePath = filepath.Join(home, storagePath[1:])
	}

	return filepath.Join(storagePath, cfg.Storage.SQLiteFile)
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
	case 's':
		return time.Duration(n) * time.Second, nil
	default:
		return 0, fmt.Errorf("invalid duration: %q (use d, h, w, m, or s suffix)", s)
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
