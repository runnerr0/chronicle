package cli

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/runnerr0/chronicle/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSearchStore(t *testing.T) *storage.SQLiteStore {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	runner := storage.NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	store, err := storage.NewSQLiteStore(db)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	return store
}

func seedSearchEvents(t *testing.T, store *storage.SQLiteStore) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()

	events := []struct {
		url, title, source, browser string
		ts                          time.Time
	}{
		{"https://lancedb.github.io/lancedb/basic/", "LanceDB Getting Started", "extension", "chrome", now.Add(-1 * time.Hour)},
		{"https://blog.example.com/chromadb-vs-lancedb", "ChromaDB vs LanceDB", "manual", "", now.Add(-48 * time.Hour)},
		{"https://github.com/golang/go", "Go Programming Language", "extension", "firefox", now.Add(-2 * time.Hour)},
		{"https://news.ycombinator.com/", "Hacker News", "extension", "chrome", now.Add(-72 * time.Hour)},
		{"https://docs.python.org/3/", "Python 3 Docs", "import", "safari", now.Add(-96 * time.Hour)},
	}

	for _, ev := range events {
		e := &storage.Event{
			URL:       ev.url,
			Title:     ev.title,
			Source:    ev.source,
			Browser:   ev.browser,
			Timestamp: ev.ts,
		}
		require.NoError(t, store.AddEvent(ctx, e))
	}
}

func captureSearchOutput(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// --- parseDuration tests ---

func TestParseDuration_Days(t *testing.T) {
	d, err := parseDuration("7d")
	require.NoError(t, err)
	assert.Equal(t, 7*24*time.Hour, d)
}

func TestParseDuration_Hours(t *testing.T) {
	d, err := parseDuration("24h")
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, d)
}

func TestParseDuration_30Days(t *testing.T) {
	d, err := parseDuration("30d")
	require.NoError(t, err)
	assert.Equal(t, 30*24*time.Hour, d)
}

func TestParseDuration_InvalidFormat(t *testing.T) {
	_, err := parseDuration("abc")
	assert.Error(t, err)
}

func TestParseDuration_Empty(t *testing.T) {
	_, err := parseDuration("")
	assert.Error(t, err)
}

// --- Search integration tests ---

func TestSearch_WithResults(t *testing.T) {
	store := setupSearchStore(t)
	seedSearchEvents(t, store)

	cmd := &SearchCommand{
		Since:   "30d",
		Limit:   10,
		globals: &GlobalFlags{},
	}

	output := captureSearchOutput(t, func() {
		err := cmd.executeWithStore(store, []string{"LanceDB"})
		require.NoError(t, err)
	})

	assert.Contains(t, output, "LanceDB Getting Started")
	assert.Contains(t, output, "lancedb.github.io")
	assert.Contains(t, output, "result")
}

func TestSearch_NoResults(t *testing.T) {
	store := setupSearchStore(t)
	seedSearchEvents(t, store)

	cmd := &SearchCommand{
		Since:   "30d",
		Limit:   10,
		globals: &GlobalFlags{},
	}

	output := captureSearchOutput(t, func() {
		err := cmd.executeWithStore(store, []string{"nonexistentterm12345"})
		require.NoError(t, err)
	})

	assert.Contains(t, output, "No results found")
}

func TestSearch_DomainFilter(t *testing.T) {
	store := setupSearchStore(t)
	seedSearchEvents(t, store)

	cmd := &SearchCommand{
		Since:   "30d",
		Domain:  []string{"github.com"},
		Limit:   10,
		globals: &GlobalFlags{},
	}

	output := captureSearchOutput(t, func() {
		err := cmd.executeWithStore(store, []string{""})
		require.NoError(t, err)
	})

	assert.Contains(t, output, "github.com")
	assert.NotContains(t, output, "lancedb.github.io")
}

func TestSearch_TimeRange_3Hours(t *testing.T) {
	store := setupSearchStore(t)
	seedSearchEvents(t, store)

	cmd := &SearchCommand{
		Since:   "3h",
		Limit:   10,
		globals: &GlobalFlags{},
	}

	output := captureSearchOutput(t, func() {
		err := cmd.executeWithStore(store, []string{""})
		require.NoError(t, err)
	})

	assert.Contains(t, output, "LanceDB Getting Started")
	assert.Contains(t, output, "Go Programming Language")
	assert.NotContains(t, output, "Hacker News")
	assert.NotContains(t, output, "Python 3 Docs")
}

func TestSearch_JSONOutput(t *testing.T) {
	store := setupSearchStore(t)
	seedSearchEvents(t, store)

	cmd := &SearchCommand{
		Since:   "30d",
		Limit:   10,
		globals: &GlobalFlags{JSON: true},
	}

	output := captureSearchOutput(t, func() {
		err := cmd.executeWithStore(store, []string{"LanceDB"})
		require.NoError(t, err)
	})

	assert.Contains(t, output, `"query"`)
	assert.Contains(t, output, `"results"`)
	assert.Contains(t, output, `"count"`)
}

func TestSearch_Pagination(t *testing.T) {
	store := setupSearchStore(t)
	seedSearchEvents(t, store)

	cmd := &SearchCommand{
		Since:   "30d",
		Limit:   2,
		Offset:  0,
		globals: &GlobalFlags{},
	}

	output := captureSearchOutput(t, func() {
		err := cmd.executeWithStore(store, []string{""})
		require.NoError(t, err)
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	resultCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && trimmed[1] == '.' {
			resultCount++
		}
	}
	assert.Equal(t, 2, resultCount, "should show exactly 2 results with limit=2")
}

func TestSearch_SemanticFallback(t *testing.T) {
	store := setupSearchStore(t)
	seedSearchEvents(t, store)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cmd := &SearchCommand{
		Since:    "30d",
		Limit:    10,
		Semantic: true,
		globals:  &GlobalFlags{},
	}

	_ = captureSearchOutput(t, func() {
		err := cmd.executeWithStore(store, []string{"LanceDB"})
		require.NoError(t, err)
	})

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	assert.Contains(t, buf.String(), "semantic search not yet implemented")
}

func TestSearch_BrowserFilter(t *testing.T) {
	store := setupSearchStore(t)
	seedSearchEvents(t, store)

	cmd := &SearchCommand{
		Since:   "30d",
		Browser: []string{"chrome"},
		Limit:   10,
		globals: &GlobalFlags{},
	}

	output := captureSearchOutput(t, func() {
		err := cmd.executeWithStore(store, []string{""})
		require.NoError(t, err)
	})

	assert.Contains(t, output, "chrome")
	assert.NotContains(t, output, "firefox")
	assert.NotContains(t, output, "safari")
}
