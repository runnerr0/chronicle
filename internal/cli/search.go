package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/runnerr0/chronicle/internal/storage"
)

// parseDuration parses a human-friendly duration string like "7d", "24h", "30d".
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration %q", s)
	}

	suffix := s[len(s)-1]
	numStr := s[:len(s)-1]

	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}

	switch suffix {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'm':
		return time.Duration(n) * time.Minute, nil
	case 's':
		return time.Duration(n) * time.Second, nil
	default:
		return 0, fmt.Errorf("unknown duration suffix %q in %q", string(suffix), s)
	}
}

// defaultDBPath returns the default Chronicle database path.
func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "chronicle.db"
	}
	return filepath.Join(home, ".config", "fabric", "chronicle", "chronicle.db")
}

// openDefaultStore opens the default SQLite store with migrations applied.
func openDefaultStore() (*storage.SQLiteStore, *sql.DB, error) {
	dbPath := defaultDBPath()

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, nil, fmt.Errorf("create db directory: %w", err)
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
		return nil, nil, fmt.Errorf("init store: %w", err)
	}

	return store, db, nil
}

// Execute implements the go-flags Commander interface for SearchCommand.
func (c *SearchCommand) Execute(args []string) error {
	store, db, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer db.Close()
	defer store.Close()

	return c.executeWithStore(store, args)
}

// executeWithStore runs the search against a provided store (for testing).
func (c *SearchCommand) executeWithStore(store *storage.SQLiteStore, args []string) error {
	query := c.Query
	if query == "" && len(args) > 0 {
		query = strings.Join(args, " ")
	}

	if c.Semantic {
		fmt.Fprintln(os.Stderr, "Note: semantic search not yet implemented, falling back to keyword search.")
	}

	now := time.Now()
	var since time.Time
	if c.Since != "" {
		dur, err := parseDuration(c.Since)
		if err != nil {
			return fmt.Errorf("invalid --since value %q: %w", c.Since, err)
		}
		since = now.Add(-dur)
	}

	var until time.Time
	if c.Until != "" {
		dur, err := parseDuration(c.Until)
		if err != nil {
			return fmt.Errorf("invalid --until value %q: %w", c.Until, err)
		}
		until = now.Add(-dur)
	}

	sq := storage.SearchQuery{
		Query:        query,
		Source:       c.Source,
		Since:        since,
		Until:        until,
		Limit:        c.Limit,
		Offset:       c.Offset,
		HasBody:      c.HasBody,
		HasEmbedding: c.HasEmbedding,
	}
	if len(c.Domain) > 0 {
		sq.Domain = c.Domain[0]
	}
	if len(c.Browser) > 0 {
		sq.Browser = c.Browser[0]
	}

	ctx := context.Background()
	results, err := store.SearchEvents(ctx, sq)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if c.globals != nil && c.globals.JSON {
		return c.printJSON(query, results)
	}
	return c.printHuman(query, results)
}

func (c *SearchCommand) printHuman(query string, results []storage.Event) error {
	if len(results) == 0 {
		if query != "" {
			fmt.Printf("No results found for %q (since %s)\n", query, c.Since)
		} else {
			fmt.Printf("No results found (since %s)\n", c.Since)
		}
		return nil
	}

	resultWord := "results"
	if len(results) == 1 {
		resultWord = "result"
	}
	if query != "" {
		fmt.Printf("Found %d %s for %q (since %s)\n\n", len(results), resultWord, query, c.Since)
	} else {
		fmt.Printf("Found %d %s (since %s)\n\n", len(results), resultWord, c.Since)
	}

	for i, e := range results {
		fmt.Printf("%d. %s", i+1+c.Offset, e.Title)
		if e.Domain != "" {
			fmt.Printf(" \u2014 %s", e.Domain)
		}
		fmt.Println()

		fmt.Printf("   %s\n", e.URL)

		ts := e.Timestamp.Local().Format("2006-01-02 15:04")
		meta := ts
		if e.Source != "" {
			meta += " \u00b7 " + e.Source
		}
		if e.Browser != "" {
			meta += " \u00b7 " + e.Browser
		}
		fmt.Printf("   %s\n", meta)

		if i < len(results)-1 {
			fmt.Println()
		}
	}

	return nil
}

type jsonResult struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Domain    string `json:"domain"`
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Browser  string `json:"browser,omitempty"`
}

type jsonSearchOutput struct {
	Count   int          `json:"count"`
	Query   string       `json:"query"`
	Results []jsonResult `json:"results"`
}

func (c *SearchCommand) printJSON(query string, results []storage.Event) error {
	out := jsonSearchOutput{
		Count:   len(results),
		Query:   query,
		Results: make([]jsonResult, len(results)),
	}

	for i, e := range results {
		out.Results[i] = jsonResult{
			ID:        e.ID,
			URL:       e.URL,
			Title:     e.Title,
			Domain:    e.Domain,
			Timestamp: e.Timestamp.UTC().Format(time.RFC3339),
			Source:    e.Source,
			Browser:  e.Browser,
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
