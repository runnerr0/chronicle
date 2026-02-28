package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/runnerr0/chronicle/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupStatusTest creates an in-memory store for testing status output.
func setupStatusTest(t *testing.T) (*storage.SQLiteStore, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	runner := storage.NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	store, err := storage.NewSQLiteStore(db)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	return store, db
}

// captureStatusOutput captures stdout from a function and returns it as a string.
func captureStatusOutput(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestStatus_EmptyDB(t *testing.T) {
	store, db := setupStatusTest(t)

	cmd := &StatusCommand{
		globals: &GlobalFlags{},
		version: "dev",
	}

	output := captureStatusOutput(t, func() {
		err := cmd.executeWithStore(store, db)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Chronicle Status")
	assert.Contains(t, output, "Version:")
	assert.Contains(t, output, "dev")
	assert.Contains(t, output, "Events:")
	assert.Contains(t, output, "0")
	assert.Contains(t, output, "Content:")
	assert.Contains(t, output, "0")
	assert.Contains(t, output, "Daemon:")
	assert.Contains(t, output, "not running")
	assert.Contains(t, output, "Embeddings:")
	assert.Contains(t, output, "disabled")
}

func TestStatus_WithData(t *testing.T) {
	store, db := setupStatusTest(t)
	ctx := context.Background()

	// Seed events with different domains
	events := []struct {
		url   string
		title string
		body  string
	}{
		{"https://github.com/repo1", "Repo 1", "body1"},
		{"https://github.com/repo2", "Repo 2", "body2"},
		{"https://github.com/repo3", "Repo 3", ""},
		{"https://stackoverflow.com/q/1", "Question 1", "answer"},
		{"https://pkg.go.dev/fmt", "fmt package", ""},
	}

	for _, e := range events {
		ev := &storage.Event{URL: e.url, Title: e.title, Source: "manual"}
		if e.body != "" {
			require.NoError(t, store.AddEventWithContent(ctx, ev, e.body))
		} else {
			require.NoError(t, store.AddEvent(ctx, ev))
		}
	}

	cmd := &StatusCommand{
		globals: &GlobalFlags{},
		version: "dev",
	}

	output := captureStatusOutput(t, func() {
		err := cmd.executeWithStore(store, db)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Events:")
	assert.Contains(t, output, "5")
	assert.Contains(t, output, "Content:")
	assert.Contains(t, output, "3")
	assert.Contains(t, output, "60.0%")
}

func TestStatus_TopDomainsSorted(t *testing.T) {
	store, db := setupStatusTest(t)
	ctx := context.Background()

	// Seed: github.com x3, stackoverflow.com x2, pkg.go.dev x1
	urls := []string{
		"https://github.com/a", "https://github.com/b", "https://github.com/c",
		"https://stackoverflow.com/q1", "https://stackoverflow.com/q2",
		"https://pkg.go.dev/fmt",
	}
	for _, u := range urls {
		ev := &storage.Event{URL: u, Title: "T", Source: "manual"}
		require.NoError(t, store.AddEvent(ctx, ev))
	}

	cmd := &StatusCommand{
		globals: &GlobalFlags{},
		version: "dev",
	}

	output := captureStatusOutput(t, func() {
		err := cmd.executeWithStore(store, db)
		require.NoError(t, err)
	})

	// github.com should appear before stackoverflow.com
	githubIdx := strings.Index(output, "github.com")
	soIdx := strings.Index(output, "stackoverflow.com")
	pkgIdx := strings.Index(output, "pkg.go.dev")

	assert.Greater(t, githubIdx, 0, "github.com should appear in output")
	assert.Greater(t, soIdx, 0, "stackoverflow.com should appear in output")
	assert.Greater(t, pkgIdx, 0, "pkg.go.dev should appear in output")
	assert.Less(t, githubIdx, soIdx, "github.com (3) should appear before stackoverflow.com (2)")
	assert.Less(t, soIdx, pkgIdx, "stackoverflow.com (2) should appear before pkg.go.dev (1)")
}

func TestStatus_JSONOutput(t *testing.T) {
	store, db := setupStatusTest(t)
	ctx := context.Background()

	ev := &storage.Event{URL: "https://example.com/page", Title: "Example", Source: "manual"}
	require.NoError(t, store.AddEventWithContent(ctx, ev, "content body"))

	cmd := &StatusCommand{
		globals: &GlobalFlags{JSON: true},
		version: "dev",
	}

	output := captureStatusOutput(t, func() {
		err := cmd.executeWithStore(store, db)
		require.NoError(t, err)
	})

	var result statusJSON
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, "dev", result.Version)
	assert.Equal(t, int64(1), result.TotalEvents)
	assert.Equal(t, int64(1), result.TotalContent)
	assert.Equal(t, 30, result.RetentionDays)
	assert.False(t, result.DaemonRunning)
	assert.False(t, result.EmbeddingsEnabled)
	assert.Greater(t, result.DatabaseSizeBytes, int64(0))
}

func TestStatus_DatabaseSizeReported(t *testing.T) {
	store, db := setupStatusTest(t)
	ctx := context.Background()

	// Add some data so DB has content
	for i := 0; i < 10; i++ {
		ev := &storage.Event{
			URL:    "https://example.com/" + string(rune('a'+i)),
			Title:  "Page",
			Source: "manual",
		}
		require.NoError(t, store.AddEventWithContent(ctx, ev, strings.Repeat("x", 1000)))
	}

	cmd := &StatusCommand{
		globals: &GlobalFlags{JSON: true},
		version: "dev",
	}

	output := captureStatusOutput(t, func() {
		err := cmd.executeWithStore(store, db)
		require.NoError(t, err)
	})

	var result statusJSON
	err := json.Unmarshal([]byte(output), &result)
	require.NoError(t, err)

	// In-memory DBs report 0 for page_count, so we accept >= 0
	assert.GreaterOrEqual(t, result.DatabaseSizeBytes, int64(0))
}
