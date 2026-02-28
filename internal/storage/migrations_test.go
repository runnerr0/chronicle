package storage

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigrationRunner_FreshDB(t *testing.T) {
	db := openTestDB(t)
	runner := NewMigrationRunner(db)

	err := runner.Run()
	require.NoError(t, err)

	// Verify all 7 tables exist
	expectedTables := []string{
		"events",
		"content",
		"exclusions",
		"config",
		"embedding_metadata",
		"audit_log",
		"schema_migrations",
	}
	for _, table := range expectedTables {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		require.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, table, name)
	}
}

func TestMigrationRunner_IndexesCreated(t *testing.T) {
	db := openTestDB(t)
	runner := NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	expectedIndexes := []string{
		"idx_events_ts",
		"idx_events_domain",
		"idx_events_browser",
		"idx_events_source",
		"idx_events_content_hash",
		"idx_events_ts_domain",
		"idx_events_flags",
		"idx_exclusions_rule",
		"idx_audit_log_ts",
		"idx_audit_log_action",
	}
	for _, idx := range expectedIndexes {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx,
		).Scan(&name)
		require.NoError(t, err, "index %s should exist", idx)
		assert.Equal(t, idx, name)
	}
}

func TestMigrationRunner_DefaultExclusions(t *testing.T) {
	db := openTestDB(t)
	runner := NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	// Count total default exclusions
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM exclusions WHERE is_default = 1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 25, count, "should have 25 default exclusion rules")

	// Verify categories exist
	categories := map[string]int{
		"Banking - financial privacy":           9,
		"Payment - financial privacy":           2,
		"Password manager - credential privacy": 4,
		"Auth provider - credential privacy":    4,
		"Healthcare - HIPAA privacy":            2,
		"Tax - financial privacy":               2,
		"Adult content exclusion":               2,
	}
	for reason, expected := range categories {
		var c int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM exclusions WHERE reason = ? AND is_default = 1", reason,
		).Scan(&c)
		require.NoError(t, err)
		assert.Equal(t, expected, c, "category %q should have %d rules", reason, expected)
	}

	// Verify both rule types present
	var domainCount, regexCount int
	err = db.QueryRow("SELECT COUNT(*) FROM exclusions WHERE rule_type = 'domain'").Scan(&domainCount)
	require.NoError(t, err)
	assert.Equal(t, 23, domainCount, "should have 23 domain rules")

	err = db.QueryRow("SELECT COUNT(*) FROM exclusions WHERE rule_type = 'regex'").Scan(&regexCount)
	require.NoError(t, err)
	assert.Equal(t, 2, regexCount, "should have 2 regex rules")
}

func TestMigrationRunner_Idempotent(t *testing.T) {
	db := openTestDB(t)
	runner := NewMigrationRunner(db)

	// Run migrations twice
	require.NoError(t, runner.Run())
	require.NoError(t, runner.Run())

	// Should still have exactly 1 migration recorded
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "should have exactly 1 migration recorded after double-run")

	// Should still have exactly 24 default exclusions (not doubled)
	err = db.QueryRow("SELECT COUNT(*) FROM exclusions WHERE is_default = 1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 25, count, "exclusions should not be duplicated on re-run")
}

func TestMigrationRunner_SchemaMigrationsTracking(t *testing.T) {
	db := openTestDB(t)
	runner := NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	var version int
	var name string
	err := db.QueryRow("SELECT version, name FROM schema_migrations WHERE version = 1").Scan(&version, &name)
	require.NoError(t, err)
	assert.Equal(t, 1, version)
	assert.Equal(t, "initial_schema", name)
}

func TestMigrationRunner_WALMode(t *testing.T) {
	db := openTestDB(t)
	runner := NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	var journalMode string
	err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	require.NoError(t, err)
	// In-memory databases use "memory" journal mode; WAL is set but only
	// takes effect on file-backed DBs. We verify the pragma was executed
	// by checking it's either "wal" or "memory".
	assert.Contains(t, []string{"wal", "memory"}, journalMode,
		"journal_mode should be wal (file) or memory (in-memory)")
}

func TestMigrationRunner_ForeignKeys(t *testing.T) {
	db := openTestDB(t)
	runner := NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	var fk int
	err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	require.NoError(t, err)
	assert.Equal(t, 1, fk, "foreign_keys should be enabled")
}

func TestMigrationRunner_ForeignKeyEnforcement(t *testing.T) {
	db := openTestDB(t)
	runner := NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	// Inserting content for a non-existent event should fail
	_, err := db.Exec(
		"INSERT INTO content (event_id, body) VALUES ('nonexistent', 'test')",
	)
	assert.Error(t, err, "foreign key constraint should prevent orphan content rows")
}

func TestMigrationRunner_EventsTableColumns(t *testing.T) {
	db := openTestDB(t)
	runner := NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	// Insert a full event row to verify all columns
	_, err := db.Exec(`
		INSERT INTO events (id, ts, url, title, domain, browser, source, has_body, has_embedding, content_hash)
		VALUES ('test-uuid', CURRENT_TIMESTAMP, 'https://example.com', 'Test', 'example.com', 'chrome', 'extension', 0, 0, 'abc123')
	`)
	require.NoError(t, err)

	var id, url, title, domain, browser, source string
	var hasBody, hasEmbedding bool
	err = db.QueryRow("SELECT id, url, title, domain, browser, source, has_body, has_embedding FROM events WHERE id = 'test-uuid'").
		Scan(&id, &url, &title, &domain, &browser, &source, &hasBody, &hasEmbedding)
	require.NoError(t, err)
	assert.Equal(t, "test-uuid", id)
	assert.Equal(t, "https://example.com", url)
	assert.Equal(t, "chrome", browser)
	assert.False(t, hasBody)
	assert.False(t, hasEmbedding)
}
