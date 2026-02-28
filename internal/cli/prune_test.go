package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/runnerr0/chronicle/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupPruneTest creates a migrated in-memory DB, inserts seed events, and
// returns a PruneCommand wired to that store.
func setupPruneTest(t *testing.T, oldCount, recentCount int) (*PruneCommand, *storage.SQLiteStore) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	runner := storage.NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	store, err := storage.NewSQLiteStore(db)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	now := time.Now()

	// Insert old events (60 days ago)
	for i := 0; i < oldCount; i++ {
		e := &storage.Event{
			URL:       fmt.Sprintf("https://old%d.com/page", i),
			Title:     fmt.Sprintf("Old Event %d", i),
			Source:    "extension",
			Timestamp: now.Add(-60 * 24 * time.Hour),
		}
		require.NoError(t, store.AddEvent(ctx, e))
	}

	// Insert recent events (1 hour ago)
	for i := 0; i < recentCount; i++ {
		e := &storage.Event{
			URL:       fmt.Sprintf("https://recent%d.com/page", i),
			Title:     fmt.Sprintf("Recent Event %d", i),
			Source:    "extension",
			Timestamp: now.Add(-1 * time.Hour),
		}
		require.NoError(t, store.AddEvent(ctx, e))
	}

	globals := &GlobalFlags{}
	cmd := &PruneCommand{
		globals: globals,
		version: "test",
		store:   store,
	}

	return cmd, store
}

// --- Prune with default TTL (30d) ---

func TestPrune_DefaultTTL(t *testing.T) {
	cmd, store := setupPruneTest(t, 5, 3)
	cmd.Force = true

	output := captureOutput(t, func() {
		err := cmd.Execute(nil)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Pruned 5 events")

	ctx := context.Background()
	stats, err := store.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.TotalEvents)
}

// --- Prune with custom --older-than ---

func TestPrune_CustomOlderThan(t *testing.T) {
	cmd, store := setupPruneTest(t, 5, 3)
	cmd.OlderThan = "7d"
	cmd.Force = true

	output := captureOutput(t, func() {
		err := cmd.Execute(nil)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Pruned 5 events")
	assert.Contains(t, output, "7 days")

	ctx := context.Background()
	stats, err := store.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.TotalEvents)
}

// --- Dry run shows count without deleting ---

func TestPrune_DryRun(t *testing.T) {
	cmd, store := setupPruneTest(t, 5, 3)
	cmd.DryRun = true

	output := captureOutput(t, func() {
		err := cmd.Execute(nil)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "[DRY RUN]")
	assert.Contains(t, output, "5 events")

	// Verify nothing was deleted
	ctx := context.Background()
	stats, err := store.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(8), stats.TotalEvents)
}

// --- Force skips confirmation ---

func TestPrune_ForceSkipsConfirmation(t *testing.T) {
	cmd, _ := setupPruneTest(t, 5, 3)
	cmd.Force = true

	output := captureOutput(t, func() {
		err := cmd.Execute(nil)
		require.NoError(t, err)
	})

	assert.NotContains(t, output, "Proceed?")
	assert.Contains(t, output, "Pruned 5 events")
}

// --- Confirmation prompt: user says yes ---

func TestPrune_ConfirmationYes(t *testing.T) {
	cmd, store := setupPruneTest(t, 5, 3)
	cmd.stdin = strings.NewReader("y\n")

	output := captureOutput(t, func() {
		err := cmd.Execute(nil)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Proceed?")
	assert.Contains(t, output, "Pruned 5 events")

	ctx := context.Background()
	stats, err := store.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.TotalEvents)
}

// --- Confirmation prompt: user says no ---

func TestPrune_ConfirmationNo(t *testing.T) {
	cmd, store := setupPruneTest(t, 5, 3)
	cmd.stdin = strings.NewReader("n\n")

	output := captureOutput(t, func() {
		err := cmd.Execute(nil)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Proceed?")
	assert.Contains(t, output, "Aborted")

	ctx := context.Background()
	stats, err := store.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(8), stats.TotalEvents)
}

// --- JSON output ---

func TestPrune_JSONOutput(t *testing.T) {
	cmd, _ := setupPruneTest(t, 5, 3)
	cmd.Force = true
	cmd.globals.JSON = true

	output := captureOutput(t, func() {
		err := cmd.Execute(nil)
		require.NoError(t, err)
	})

	var result map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result)
	require.NoError(t, err, "output should be valid JSON: %s", output)

	assert.Equal(t, float64(5), result["pruned"])
	assert.Equal(t, false, result["dry_run"])
	assert.Contains(t, result, "older_than")
}

// --- JSON output for dry run ---

func TestPrune_JSONDryRun(t *testing.T) {
	cmd, _ := setupPruneTest(t, 5, 3)
	cmd.DryRun = true
	cmd.globals.JSON = true

	output := captureOutput(t, func() {
		err := cmd.Execute(nil)
		require.NoError(t, err)
	})

	var result map[string]interface{}
	err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result)
	require.NoError(t, err)

	assert.Equal(t, float64(5), result["pruned"])
	assert.Equal(t, true, result["dry_run"])
}

// --- Nothing to prune ---

func TestPrune_NothingToPrune(t *testing.T) {
	cmd, _ := setupPruneTest(t, 0, 3)
	cmd.Force = true

	output := captureOutput(t, func() {
		err := cmd.Execute(nil)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "No events to prune")
}

// --- Invalid --older-than ---

func TestPrune_InvalidOlderThan(t *testing.T) {
	cmd, _ := setupPruneTest(t, 5, 3)
	cmd.OlderThan = "invalid"

	err := cmd.Execute(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid duration")
}

// --- parseDuration tests ---

func TestPruneParseDuration_Days(t *testing.T) {
	d, err := parseDuration("30d")
	require.NoError(t, err)
	assert.Equal(t, 30*24*time.Hour, d)
}

func TestPruneParseDuration_Hours(t *testing.T) {
	d, err := parseDuration("24h")
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, d)
}

func TestPruneParseDuration_Weeks(t *testing.T) {
	d, err := parseDuration("2w")
	require.NoError(t, err)
	assert.Equal(t, 14*24*time.Hour, d)
}

func TestPruneParseDuration_Invalid(t *testing.T) {
	_, err := parseDuration("abc")
	assert.Error(t, err)
}

// --- Flag parsing ---

func TestPruneForceFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"prune", "--force"})
	require.NoError(t, err)
	assert.True(t, c.Prune.Force)
}

func TestPruneDryRunAndOlderThanFlags(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"prune", "--dry-run", "--older-than", "14d"})
	require.NoError(t, err)
	assert.True(t, c.Prune.DryRun)
	assert.Equal(t, "14d", c.Prune.OlderThan)
}
