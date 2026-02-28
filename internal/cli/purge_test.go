package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runnerr0/chronicle/internal/storage"
)

// openTestDB creates a migrated in-memory SQLite database for testing.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	runner := storage.NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	return db
}

func TestPurge_WithoutAllFlag_Errors(t *testing.T) {
	err := RunWithArgs("test", []string{"purge"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "purge requires --all flag for safety")
}

func TestPurge_WithAllAndForce_Succeeds(t *testing.T) {
	db := openTestDB(t)

	// Seed data
	_, err := db.Exec(`INSERT INTO events (id, ts, url, title, domain, browser, source, has_body, has_embedding)
		VALUES ('CHR-test1', '2025-01-01T00:00:00Z', 'https://example.com', 'Test', 'example.com', 'chrome', 'manual', 0, 0)`)
	require.NoError(t, err)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &PurgeCommand{
		All:   true,
		Force: true,
		globals: &GlobalFlags{},
	}
	cmd.setDB(db)

	err = cmd.Execute(nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	require.NoError(t, err)
	assert.Contains(t, output, "Purged all data")

	// Verify DB is empty
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestPurge_JSONOutput(t *testing.T) {
	db := openTestDB(t)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &PurgeCommand{
		All:   true,
		Force: true,
		globals: &GlobalFlags{JSON: true},
	}
	cmd.setDB(db)

	err := cmd.Execute(nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "output should be valid JSON: %s", output)
	assert.Equal(t, true, result["purged"])
	assert.Equal(t, "all data deleted", result["message"])
}

func TestPurge_DBIsEmptyAfterPurge(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Seed multiple events and content
	for i, id := range []string{"CHR-aa01", "CHR-bb02", "CHR-cc03"} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO events (id, ts, url, title, domain, browser, source, has_body, has_embedding)
			 VALUES (?, '2025-01-01T00:00:00Z', ?, 'Test', 'example.com', 'chrome', 'manual', 1, 0)`,
			id, "https://example.com/"+string(rune('a'+i)))
		require.NoError(t, err)

		_, err = db.ExecContext(ctx,
			`INSERT INTO content (event_id, body, byte_size) VALUES (?, ?, ?)`,
			id, "body content", 12)
		require.NoError(t, err)
	}

	// Verify data exists
	var eventCount, contentCount int
	db.QueryRow("SELECT COUNT(*) FROM events").Scan(&eventCount)
	db.QueryRow("SELECT COUNT(*) FROM content").Scan(&contentCount)
	assert.Equal(t, 3, eventCount)
	assert.Equal(t, 3, contentCount)

	// Run purge
	cmd := &PurgeCommand{
		All:   true,
		Force: true,
		globals: &GlobalFlags{},
	}
	cmd.setDB(db)

	// Capture stdout to suppress output
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	err := cmd.Execute(nil)
	w.Close()
	os.Stdout = old

	require.NoError(t, err)

	// Verify everything is gone
	db.QueryRow("SELECT COUNT(*) FROM events").Scan(&eventCount)
	db.QueryRow("SELECT COUNT(*) FROM content").Scan(&contentCount)
	assert.Equal(t, 0, eventCount, "events table should be empty")
	assert.Equal(t, 0, contentCount, "content table should be empty")
}
