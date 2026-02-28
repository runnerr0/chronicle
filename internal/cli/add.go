package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/runnerr0/chronicle/internal/storage"
)

// Execute implements the go-flags Commander interface for AddCommand.
func (c *AddCommand) Execute(args []string) error {
	if c.URL == "" {
		return fmt.Errorf("--url is required for add command")
	}
	if c.Title == "" {
		return fmt.Errorf("--title is required for add command")
	}

	store, db, err := c.openStore()
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer store.Close()
	defer db.Close()

	return c.executeWithStore(store)
}

// openStore resolves the config path and opens the SQLite store.
func (c *AddCommand) openStore() (*storage.SQLiteStore, *sql.DB, error) {
	configPath := c.globals.Config
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, err
		}
		configPath = home + "/.config/fabric/chronicle"
	}

	dbPath := configPath + "/chronicle.db"
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, nil, err
	}

	runner := storage.NewMigrationRunner(db)
	if err := runner.Run(); err != nil {
		db.Close()
		return nil, nil, err
	}

	store, err := storage.NewSQLiteStore(db)
	if err != nil {
		db.Close()
		return nil, nil, err
	}

	return store, db, nil
}

// executeWithStore runs the add logic against a provided store (used by tests).
func (c *AddCommand) executeWithStore(store *storage.SQLiteStore) error {
	// Validate URL format
	parsed, err := url.ParseRequestURI(c.URL)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("invalid URL: %s", c.URL)
	}

	// Body and body-file are mutually exclusive
	if c.Body != "" && c.BodyFile != "" {
		return fmt.Errorf("--body and --body-file are mutually exclusive")
	}

	// Read body from file if specified
	body := c.Body
	if c.BodyFile != "" {
		data, err := os.ReadFile(c.BodyFile)
		if err != nil {
			return fmt.Errorf("reading body file: %w", err)
		}
		body = string(data)
	}

	ctx := context.Background()

	event := &storage.Event{
		URL:       c.URL,
		Title:     c.Title,
		Browser:   c.BrowserName,
		Source:    "manual",
		Timestamp: time.Now(),
	}

	// Compute content hash for dedup if body is present
	if body != "" {
		hash := sha256.Sum256([]byte(body))
		event.ContentHash = fmt.Sprintf("%x", hash)
	}

	// Check exclusion before calling store (store silently skips, but we want
	// an explicit error for the CLI user)
	domain := parsed.Hostname()
	if store.IsExcluded(domain) {
		return fmt.Errorf("domain %q is excluded by exclusion rules", domain)
	}

	if body != "" {
		err = store.AddEventWithContent(ctx, event, body)
	} else {
		err = store.AddEvent(ctx, event)
	}
	if err != nil {
		return fmt.Errorf("storing event: %w", err)
	}

	// Output confirmation
	if c.globals.JSON {
		out := map[string]interface{}{
			"id":    event.ID,
			"url":   event.URL,
			"title": event.Title,
			"ts":    event.Timestamp.Format(time.RFC3339),
			"body":  body != "",
			"embed": false,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	hasBody := "no"
	if body != "" {
		hasBody = "yes"
	}

	fmt.Printf("Added event %s (%s)\n", event.ID, event.Timestamp.Format(time.RFC3339))
	fmt.Printf("  URL: %s\n", event.URL)
	fmt.Printf("  Title: %s\n", event.Title)
	fmt.Printf("  Body: %s\n", hasBody)
	fmt.Printf("  Embedding: %s\n", "no")

	return nil
}
