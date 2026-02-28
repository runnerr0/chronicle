package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runnerr0/chronicle/internal/storage"
)

// setupOpenTestDB creates a temp DB with a seeded event+content for open command tests.
// Returns the DB path and the event ID.
func setupOpenTestDB(t *testing.T) (string, string) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "chronicle-open-test-*.db")
	require.NoError(t, err)
	dbPath := tmpFile.Name()
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(dbPath) })

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	require.NoError(t, err)
	defer db.Close()

	runner := storage.NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	store, err := storage.NewSQLiteStore(db)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	event := &storage.Event{
		URL:       "https://lancedb.github.io/lancedb/basic/",
		Title:     "LanceDB Getting Started",
		Source:    "extension",
		Browser:   "chrome",
		Timestamp: time.Date(2026, 2, 27, 10, 0, 5, 0, time.UTC),
	}
	body := "This is the page body content for testing."
	require.NoError(t, store.AddEventWithContent(ctx, event, body))

	return dbPath, event.ID
}

func captureOpenOutput(t *testing.T, args []string) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunWithArgs("test", args)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String(), err
}

func TestOpenExistingEventShowsAllFields(t *testing.T) {
	dbPath, eventID := setupOpenTestDB(t)

	output, err := captureOpenOutput(t, []string{"open", "--id", eventID, "--config", "/dev/null", "--db-path", dbPath})
	require.NoError(t, err)

	assert.Contains(t, output, eventID)
	assert.Contains(t, output, "LanceDB Getting Started")
	assert.Contains(t, output, "https://lancedb.github.io/lancedb/basic/")
	assert.Contains(t, output, "lancedb.github.io")
	assert.Contains(t, output, "2026-02-27")
	assert.Contains(t, output, "extension")
	assert.Contains(t, output, "chrome")
	assert.Contains(t, output, "This is the page body content for testing.")
}

func TestOpenFormatURL(t *testing.T) {
	dbPath, eventID := setupOpenTestDB(t)

	output, err := captureOpenOutput(t, []string{"open", "--id", eventID, "--format", "url", "--db-path", dbPath})
	require.NoError(t, err)

	trimmed := strings.TrimSpace(output)
	assert.Equal(t, "https://lancedb.github.io/lancedb/basic/", trimmed)
}

func TestOpenFormatTitle(t *testing.T) {
	dbPath, eventID := setupOpenTestDB(t)

	output, err := captureOpenOutput(t, []string{"open", "--id", eventID, "--format", "title", "--db-path", dbPath})
	require.NoError(t, err)

	trimmed := strings.TrimSpace(output)
	assert.Equal(t, "LanceDB Getting Started", trimmed)
}

func TestOpenFormatBody(t *testing.T) {
	dbPath, eventID := setupOpenTestDB(t)

	output, err := captureOpenOutput(t, []string{"open", "--id", eventID, "--format", "body", "--db-path", dbPath})
	require.NoError(t, err)

	trimmed := strings.TrimSpace(output)
	assert.Equal(t, "This is the page body content for testing.", trimmed)
}

func TestOpenFormatMetadata(t *testing.T) {
	dbPath, eventID := setupOpenTestDB(t)

	output, err := captureOpenOutput(t, []string{"open", "--id", eventID, "--format", "metadata", "--db-path", dbPath})
	require.NoError(t, err)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &meta))

	assert.Equal(t, eventID, meta["id"])
	assert.Equal(t, "LanceDB Getting Started", meta["title"])
	assert.Equal(t, "https://lancedb.github.io/lancedb/basic/", meta["url"])
	assert.Equal(t, "lancedb.github.io", meta["domain"])
	assert.Equal(t, "extension", meta["source"])
	assert.Equal(t, "chrome", meta["browser"])
}

func TestOpenNonexistentIDReturnsError(t *testing.T) {
	dbPath, _ := setupOpenTestDB(t)

	_, err := captureOpenOutput(t, []string{"open", "--id", "CHR-nonexistent", "--db-path", dbPath})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestOpenJSONOutput(t *testing.T) {
	dbPath, eventID := setupOpenTestDB(t)

	output, err := captureOpenOutput(t, []string{"--json", "open", "--id", eventID, "--db-path", dbPath})
	require.NoError(t, err)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &result))

	assert.Equal(t, eventID, result["id"])
	assert.Equal(t, "LanceDB Getting Started", result["title"])
	assert.Equal(t, "https://lancedb.github.io/lancedb/basic/", result["url"])
	assert.Equal(t, "This is the page body content for testing.", result["body"])
}

func TestOpenMissingID(t *testing.T) {
	err := RunWithArgs("test", []string{"open"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--id is required")
}

func TestOpenEventWithoutContent(t *testing.T) {
	// Create event without content
	tmpFile, err := os.CreateTemp("", "chronicle-open-nobody-*.db")
	require.NoError(t, err)
	dbPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	require.NoError(t, err)
	defer db.Close()

	runner := storage.NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	store, err := storage.NewSQLiteStore(db)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	event := &storage.Event{
		URL:     "https://example.com",
		Title:   "No Body Page",
		Source:  "manual",
		Browser: "firefox",
	}
	require.NoError(t, store.AddEvent(ctx, event))

	output, err := captureOpenOutput(t, []string{"open", "--id", event.ID, "--db-path", dbPath})
	require.NoError(t, err)
	assert.Contains(t, output, "No content captured")
}
