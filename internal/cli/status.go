package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/runnerr0/chronicle/internal/storage"
)

// statusJSON is the JSON output structure for the status command.
type statusJSON struct {
	Version           string            `json:"version"`
	DatabasePath      string            `json:"database_path"`
	DatabaseSizeBytes int64             `json:"database_size_bytes"`
	TotalEvents       int64             `json:"total_events"`
	TotalContent      int64             `json:"total_content"`
	OldestEvent       string            `json:"oldest_event,omitempty"`
	NewestEvent       string            `json:"newest_event,omitempty"`
	RetentionDays     int               `json:"retention_days"`
	TopDomains        []domainCountJSON `json:"top_domains"`
	DaemonRunning     bool              `json:"daemon_running"`
	EmbeddingsEnabled bool              `json:"embeddings_enabled"`
}

type domainCountJSON struct {
	Domain string `json:"domain"`
	Count  int64  `json:"count"`
}

// Execute implements the go-flags Commander interface for StatusCommand.
func (c *StatusCommand) Execute(args []string) error {
	store, db, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer db.Close()
	defer store.Close()

	return c.executeWithStore(store, db)
}

// executeWithStore runs status against a provided store and db (for testing).
func (c *StatusCommand) executeWithStore(store *storage.SQLiteStore, db *sql.DB) error {
	ctx := context.Background()

	stats, err := store.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("get stats: %w", err)
	}

	// Database size
	dbPath := defaultDBPath()
	dbSize := getDatabaseSize(db, dbPath)

	// Daemon check
	daemonRunning := checkDaemon()

	// Retention (default 30 days)
	retentionDays := 30

	if c.globals != nil && c.globals.JSON {
		return c.printStatusJSON(stats, dbPath, dbSize, daemonRunning, retentionDays)
	}
	return c.printStatusHuman(stats, dbPath, dbSize, daemonRunning, retentionDays)
}

func (c *StatusCommand) printStatusHuman(stats *storage.Stats, dbPath string, dbSize int64, daemonRunning bool, retentionDays int) error {
	fmt.Println("Chronicle Status")
	fmt.Println("================")
	fmt.Printf("Version:       %s\n", c.version)
	fmt.Printf("Database:      %s (%s)\n", dbPath, formatBytes(dbSize))
	fmt.Printf("Events:        %s\n", formatNumber(stats.TotalEvents))

	// Content with percentage
	if stats.TotalEvents > 0 {
		pct := float64(stats.TotalContent) / float64(stats.TotalEvents) * 100
		fmt.Printf("Content:       %s (%.1f%%)\n", formatNumber(stats.TotalContent), pct)
	} else {
		fmt.Printf("Content:       %s\n", formatNumber(stats.TotalContent))
	}

	// Time range
	if stats.TotalEvents > 0 {
		fmt.Printf("Oldest:        %s\n", stats.OldestEvent.Local().Format("2006-01-02"))
		fmt.Printf("Newest:        %s\n", stats.NewestEvent.Local().Format("2006-01-02"))
	}

	fmt.Printf("Retention:     %d days\n", retentionDays)

	// Top domains
	if len(stats.TopDomains) > 0 {
		fmt.Println()
		fmt.Println("Top Domains:")
		for _, d := range stats.TopDomains {
			fmt.Printf("  %-20s %s\n", d.Domain, formatNumber(d.Count))
		}
	}

	fmt.Println()
	if daemonRunning {
		fmt.Println("Daemon:        running")
	} else {
		fmt.Println("Daemon:        not running")
	}
	fmt.Println("Embeddings:    disabled")

	return nil
}

func (c *StatusCommand) printStatusJSON(stats *storage.Stats, dbPath string, dbSize int64, daemonRunning bool, retentionDays int) error {
	out := statusJSON{
		Version:           c.version,
		DatabasePath:      dbPath,
		DatabaseSizeBytes: dbSize,
		TotalEvents:       stats.TotalEvents,
		TotalContent:      stats.TotalContent,
		RetentionDays:     retentionDays,
		TopDomains:        make([]domainCountJSON, len(stats.TopDomains)),
		DaemonRunning:     daemonRunning,
		EmbeddingsEnabled: false,
	}

	if stats.TotalEvents > 0 {
		out.OldestEvent = stats.OldestEvent.UTC().Format(time.RFC3339)
		out.NewestEvent = stats.NewestEvent.UTC().Format(time.RFC3339)
	}

	for i, d := range stats.TopDomains {
		out.TopDomains[i] = domainCountJSON{Domain: d.Domain, Count: d.Count}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// getDatabaseSize returns the database file size in bytes.
// For on-disk databases, it uses os.Stat. For in-memory databases,
// it queries page_count * page_size.
func getDatabaseSize(db *sql.DB, dbPath string) int64 {
	// Try file stat first
	if info, err := os.Stat(dbPath); err == nil {
		return info.Size()
	}

	// Fallback: query SQLite for in-memory or unavailable file
	var pageCount, pageSize int64
	if err := db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return 0
	}
	if err := db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0
	}
	return pageCount * pageSize
}

// checkDaemon attempts an HTTP GET to the default daemon endpoint.
// Returns true if the daemon responds within 1 second.
func checkDaemon() bool {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get("http://localhost:7773/status")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// formatBytes formats a byte count into a human-readable string.
func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// formatNumber formats an int64 with comma separators.
func formatNumber(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		result.WriteString(s[:remainder])
		if len(s) > remainder {
			result.WriteString(",")
		}
	}
	for i := remainder; i < len(s); i += 3 {
		if i > remainder {
			result.WriteString(",")
		}
		result.WriteString(s[i : i+3])
	}
	return result.String()
}
