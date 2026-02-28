package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/runnerr0/chronicle/internal/storage"
)

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
