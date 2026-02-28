package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runnerr0/chronicle/internal/storage"
)

// testStore creates a temporary SQLite database with migrations applied and
// returns a storage.Store along with a cleanup function.
func testStore(t *testing.T) (*storage.SQLiteStore, func()) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	require.NoError(t, err)

	runner := storage.NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	store, err := storage.NewSQLiteStore(db)
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		db.Close()
	}
	return store, cleanup
}

func TestAddCommand_BasicEvent(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	cmd := &AddCommand{
		URL:         "https://example.com/article",
		Title:       "Great Article",
		BrowserName: "manual",
		globals:     &GlobalFlags{},
	}

	err := cmd.executeWithStore(store)
	require.NoError(t, err)

	// Verify event was stored
	stats, err := store.GetStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalEvents)
}

func TestAddCommand_WithInlineBody(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	cmd := &AddCommand{
		URL:         "https://example.com/page",
		Title:       "Test Page",
		Body:        "This is the body content of the page.",
		BrowserName: "manual",
		globals:     &GlobalFlags{},
	}

	err := cmd.executeWithStore(store)
	require.NoError(t, err)

	stats, err := store.GetStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalEvents)
	assert.Equal(t, int64(1), stats.TotalContent)
}

func TestAddCommand_WithBodyFile(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// Create a temp file with body content
	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "content.md")
	err := os.WriteFile(bodyPath, []byte("# Article\nBody from file."), 0644)
	require.NoError(t, err)

	cmd := &AddCommand{
		URL:         "https://example.com/file-body",
		Title:       "File Body Test",
		BodyFile:    bodyPath,
		BrowserName: "manual",
		globals:     &GlobalFlags{},
	}

	err = cmd.executeWithStore(store)
	require.NoError(t, err)

	stats, err := store.GetStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalContent)
}

func TestAddCommand_RejectsInvalidURL(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	cmd := &AddCommand{
		URL:         "not-a-valid-url",
		Title:       "Bad URL",
		BrowserName: "manual",
		globals:     &GlobalFlags{},
	}

	err := cmd.executeWithStore(store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestAddCommand_RejectsExcludedDomain(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	// The migration seeds exclusion rules. chase.com is in the default denylist.
	cmd := &AddCommand{
		URL:         "https://chase.com/login",
		Title:       "Chase Login",
		BrowserName: "manual",
		globals:     &GlobalFlags{},
	}

	err := cmd.executeWithStore(store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "excluded")
}

func TestAddCommand_RequiresURL(t *testing.T) {
	cmd := &AddCommand{
		Title:       "No URL",
		BrowserName: "manual",
		globals:     &GlobalFlags{},
	}

	err := cmd.Execute(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--url is required")
}

func TestAddCommand_RequiresTitle(t *testing.T) {
	cmd := &AddCommand{
		URL:         "https://example.com",
		BrowserName: "manual",
		globals:     &GlobalFlags{},
	}

	err := cmd.Execute(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--title is required")
}

func TestAddCommand_BodyAndBodyFileMutuallyExclusive(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	cmd := &AddCommand{
		URL:         "https://example.com/both",
		Title:       "Both Body Flags",
		Body:        "inline text",
		BodyFile:    "/tmp/nonexistent.md",
		BrowserName: "manual",
		globals:     &GlobalFlags{},
	}

	err := cmd.executeWithStore(store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestAddCommand_ContentHash(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	body := "This is test content for hashing."
	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))

	cmd := &AddCommand{
		URL:         "https://example.com/hash-test",
		Title:       "Hash Test",
		Body:        body,
		BrowserName: "manual",
		globals:     &GlobalFlags{},
	}

	err := cmd.executeWithStore(store)
	require.NoError(t, err)

	// Search for the event and verify content_hash
	events, err := store.SearchEvents(context.Background(), storage.SearchQuery{
		Query: "Hash Test",
		Limit: 1,
	})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, expectedHash, events[0].ContentHash)
}

func TestAddCommand_SetsSourceManual(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	cmd := &AddCommand{
		URL:         "https://example.com/source-test",
		Title:       "Source Test",
		BrowserName: "firefox",
		globals:     &GlobalFlags{},
	}

	err := cmd.executeWithStore(store)
	require.NoError(t, err)

	events, err := store.SearchEvents(context.Background(), storage.SearchQuery{
		Query: "Source Test",
		Limit: 1,
	})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "manual", events[0].Source)
	assert.Equal(t, "firefox", events[0].Browser)
}

func TestAddCommand_BodyFileNotFound(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	cmd := &AddCommand{
		URL:         "https://example.com/no-file",
		Title:       "Missing File",
		BodyFile:    "/tmp/definitely-does-not-exist-12345.md",
		BrowserName: "manual",
		globals:     &GlobalFlags{},
	}

	err := cmd.executeWithStore(store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading body file")
}

func TestAddCommand_ExtractsDomain(t *testing.T) {
	store, cleanup := testStore(t)
	defer cleanup()

	cmd := &AddCommand{
		URL:         "https://docs.github.com/en/repositories",
		Title:       "GitHub Docs",
		BrowserName: "manual",
		globals:     &GlobalFlags{},
	}

	err := cmd.executeWithStore(store)
	require.NoError(t, err)

	events, err := store.SearchEvents(context.Background(), storage.SearchQuery{
		Domain: "docs.github.com",
		Limit:  1,
	})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "docs.github.com", events[0].Domain)
}
