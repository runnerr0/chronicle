package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/runnerr0/chronicle/internal/storage"
)

// setDB allows tests to inject a database connection.
func (c *PurgeCommand) setDB(db *sql.DB) {
	c.db = db
}

// Execute implements the go-flags Commander interface for PurgeCommand.
func (c *PurgeCommand) Execute(args []string) error {
	if !c.All {
		return fmt.Errorf("purge requires --all flag for safety")
	}

	// Confirmation prompt unless --force
	if !c.Force {
		fmt.Println("\u26a0 WARNING: This will permanently delete ALL Chronicle data.")
		fmt.Println("  - All browsing events")
		fmt.Println("  - All captured content")
		fmt.Println("  - All embeddings")
		fmt.Println()
		fmt.Println("This action cannot be undone.")
		fmt.Println()
		fmt.Print(`Type "PURGE" to confirm: `)

		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return fmt.Errorf("aborted: no input received")
		}
		input := strings.TrimSpace(scanner.Text())
		if input != "PURGE" {
			return fmt.Errorf("aborted: confirmation text did not match")
		}
	}

	// Open or use injected DB
	db := c.db
	if db == nil {
		dbPath, pathErr := defaultDBPath()
		if pathErr != nil {
			return fmt.Errorf("resolve db path: %w", pathErr)
		}
		var err error
		db, err = sql.Open("sqlite3", dbPath)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		runner := storage.NewMigrationRunner(db)
		if err := runner.Run(); err != nil {
			return fmt.Errorf("run migrations: %w", err)
		}
	}

	store, err := storage.NewSQLiteStore(db)
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.PurgeAll(ctx); err != nil {
		return fmt.Errorf("purge failed: %w", err)
	}

	// Output
	if c.globals.JSON {
		out := map[string]interface{}{
			"purged":  true,
			"message": "all data deleted",
		}
		enc := json.NewEncoder(os.Stdout)
		return enc.Encode(out)
	}

	fmt.Println("Purged all data. Chronicle is empty.")
	return nil
}

// defaultDBPath returns the default Chronicle database path.
func defaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home + "/.chronicle/chronicle.db", nil
}
