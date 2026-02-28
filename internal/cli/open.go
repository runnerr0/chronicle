package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/runnerr0/chronicle/internal/config"
	"github.com/runnerr0/chronicle/internal/storage"
)

// Execute implements the go-flags Commander interface for OpenCommand.
func (c *OpenCommand) Execute(args []string) error {
	if c.ID == "" {
		return fmt.Errorf("--id is required for open command")
	}

	// Resolve DB path
	dbPath, err := resolveDBPath(c.globals)
	if err != nil {
		return err
	}

	// Open DB and run migrations
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	runner := storage.NewMigrationRunner(db)
	if err := runner.Run(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	store, err := storage.NewSQLiteStore(db)
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Get event
	event, err := store.GetEvent(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("event not found: %s", c.ID)
	}

	// Get content (may not exist)
	content, _ := store.GetContent(ctx, c.ID)
	bodyText := ""
	if content != nil {
		bodyText = content.Body
	}

	// JSON output (--json global flag)
	if c.globals.JSON {
		return c.outputJSON(event, bodyText)
	}

	// Format-specific output
	switch c.Format {
	case "url":
		fmt.Println(event.URL)
	case "title":
		fmt.Println(event.Title)
	case "body", "raw":
		if bodyText == "" {
			fmt.Println("No content captured")
		} else {
			fmt.Println(bodyText)
		}
	case "metadata":
		return c.outputMetadata(event)
	case "json":
		return c.outputJSON(event, bodyText)
	case "md":
		c.outputMarkdown(event, bodyText)
	default: // "full"
		c.outputFull(event, bodyText)
	}

	return nil
}

func (c *OpenCommand) outputFull(event *storage.Event, body string) {
	fmt.Println(event.ID)
	fmt.Printf("Title:     %s\n", event.Title)
	fmt.Printf("URL:       %s\n", event.URL)
	fmt.Printf("Domain:    %s\n", event.Domain)
	fmt.Printf("Captured:  %s\n", event.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Source:    %s\n", event.Source)
	fmt.Printf("Browser:   %s\n", event.Browser)
	fmt.Println()
	fmt.Println("--- Content ---")
	if body == "" {
		fmt.Println("No content captured")
	} else {
		fmt.Println(body)
	}
}

func (c *OpenCommand) outputMarkdown(event *storage.Event, body string) {
	fmt.Println("---")
	fmt.Printf("id: %s\n", event.ID)
	fmt.Printf("title: %s\n", event.Title)
	fmt.Printf("url: %s\n", event.URL)
	fmt.Printf("domain: %s\n", event.Domain)
	fmt.Printf("captured: %s\n", event.Timestamp.Format("2006-01-02T15:04:05Z"))
	fmt.Printf("source: %s\n", event.Source)
	fmt.Printf("browser: %s\n", event.Browser)
	fmt.Println("---")
	if body == "" {
		fmt.Println()
		fmt.Println("No content captured")
	} else {
		fmt.Println()
		fmt.Println(body)
	}
}

func (c *OpenCommand) outputMetadata(event *storage.Event) error {
	meta := map[string]interface{}{
		"id":        event.ID,
		"title":     event.Title,
		"url":       event.URL,
		"domain":    event.Domain,
		"captured":  event.Timestamp.Format("2006-01-02T15:04:05Z"),
		"source":    event.Source,
		"browser":   event.Browser,
		"has_body":  event.HasBody,
		"has_embed": event.HasEmbed,
	}
	if event.ContentHash != "" {
		meta["content_hash"] = event.ContentHash
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(meta)
}

func (c *OpenCommand) outputJSON(event *storage.Event, body string) error {
	result := map[string]interface{}{
		"id":        event.ID,
		"title":     event.Title,
		"url":       event.URL,
		"domain":    event.Domain,
		"captured":  event.Timestamp.Format("2006-01-02T15:04:05Z"),
		"source":    event.Source,
		"browser":   event.Browser,
		"has_body":  event.HasBody,
		"has_embed": event.HasEmbed,
		"body":      body,
	}
	if event.ContentHash != "" {
		result["content_hash"] = event.ContentHash
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// resolveDBPath determines the SQLite database file path.
// Priority: --db-path flag > config file > default config.
func resolveDBPath(globals *GlobalFlags) (string, error) {
	if globals.DBPath != "" {
		return globals.DBPath, nil
	}

	var cfg *config.Config
	var err error

	if globals.Config != "" {
		cfg, err = config.Load(globals.Config)
		if err != nil {
			// If config specified but unreadable, use defaults
			cfg = config.DefaultConfig()
		}
	} else {
		cfg, err = config.LoadOrCreate()
		if err != nil {
			cfg = config.DefaultConfig()
		}
	}

	storagePath := cfg.Storage.Path
	if strings.HasPrefix(storagePath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		storagePath = filepath.Join(home, storagePath[1:])
	}

	return filepath.Join(storagePath, cfg.Storage.SQLiteFile), nil
}
