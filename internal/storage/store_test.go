package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openTestStore creates a migrated in-memory Store for testing.
func openTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	runner := NewMigrationRunner(db)
	require.NoError(t, runner.Run())

	store, err := NewSQLiteStore(db)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	return store
}

// --- AddEvent + GetEvent roundtrip ---

func TestAddEvent_GetEvent_Roundtrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	event := &Event{
		URL:    "https://example.com/article",
		Title:  "Test Article",
		Source: "manual",
	}

	err := store.AddEvent(ctx, event)
	require.NoError(t, err)

	// ID should be generated with CHR- prefix
	assert.True(t, len(event.ID) > 0, "event ID should be populated")
	assert.Contains(t, event.ID, "CHR-", "event ID should have CHR- prefix")

	// Domain should be auto-extracted
	assert.Equal(t, "example.com", event.Domain)

	// Get it back
	got, err := store.GetEvent(ctx, event.ID)
	require.NoError(t, err)
	assert.Equal(t, event.ID, got.ID)
	assert.Equal(t, "https://example.com/article", got.URL)
	assert.Equal(t, "Test Article", got.Title)
	assert.Equal(t, "example.com", got.Domain)
	assert.Equal(t, "manual", got.Source)
	assert.False(t, got.Timestamp.IsZero(), "timestamp should be set")
}

func TestAddEvent_GeneratesUniqueIDs(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	e1 := &Event{URL: "https://a.com", Title: "A", Source: "manual"}
	e2 := &Event{URL: "https://b.com", Title: "B", Source: "manual"}

	require.NoError(t, store.AddEvent(ctx, e1))
	require.NoError(t, store.AddEvent(ctx, e2))

	assert.NotEqual(t, e1.ID, e2.ID, "IDs should be unique")
}

func TestAddEvent_ExtractsDomain(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	tests := []struct {
		url      string
		expected string
	}{
		{"https://www.example.com/page", "www.example.com"},
		{"http://blog.test.org/post/123", "blog.test.org"},
		{"https://example.com", "example.com"},
	}

	for _, tc := range tests {
		event := &Event{URL: tc.url, Title: "Test", Source: "manual"}
		require.NoError(t, store.AddEvent(ctx, event))
		assert.Equal(t, tc.expected, event.Domain, "domain for %s", tc.url)
	}
}

func TestAddEvent_WithContent(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	event := &Event{
		URL:    "https://example.com/article",
		Title:  "Content Article",
		Source: "extension",
	}

	err := store.AddEventWithContent(ctx, event, "This is the article body in markdown.")
	require.NoError(t, err)

	// Verify content was stored
	content, err := store.GetContent(ctx, event.ID)
	require.NoError(t, err)
	assert.Equal(t, event.ID, content.EventID)
	assert.Equal(t, "This is the article body in markdown.", content.Body)
}

func TestGetEvent_NotFound(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	got, err := store.GetEvent(ctx, "CHR-nonexistent")
	assert.Error(t, err)
	assert.Nil(t, got)
}

// --- SearchEvents ---

func TestSearchEvents_ByQuery(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// Seed events for FTS
	e1 := &Event{URL: "https://golang.org/doc", Title: "Golang Programming Language", Source: "manual"}
	e2 := &Event{URL: "https://rust-lang.org", Title: "Rust Programming Language", Source: "manual"}
	e3 := &Event{URL: "https://python.org", Title: "Python Language", Source: "manual"}

	require.NoError(t, store.AddEvent(ctx, e1))
	require.NoError(t, store.AddEvent(ctx, e2))
	require.NoError(t, store.AddEvent(ctx, e3))

	// Search for "Golang"
	results, err := store.SearchEvents(ctx, SearchQuery{Query: "Golang", Limit: 10})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1, "should find at least one result for 'Golang'")
	assert.Equal(t, "Golang Programming Language", results[0].Title)
}

func TestSearchEvents_ByDomain(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	e1 := &Event{URL: "https://example.com/a", Title: "A", Source: "manual"}
	e2 := &Event{URL: "https://other.com/b", Title: "B", Source: "manual"}
	e3 := &Event{URL: "https://example.com/c", Title: "C", Source: "manual"}

	require.NoError(t, store.AddEvent(ctx, e1))
	require.NoError(t, store.AddEvent(ctx, e2))
	require.NoError(t, store.AddEvent(ctx, e3))

	results, err := store.SearchEvents(ctx, SearchQuery{Domain: "example.com", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 2, len(results), "should find 2 events from example.com")
	for _, r := range results {
		assert.Equal(t, "example.com", r.Domain)
	}
}

func TestSearchEvents_BySource(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	e1 := &Event{URL: "https://a.com", Title: "A", Source: "extension"}
	e2 := &Event{URL: "https://b.com", Title: "B", Source: "manual"}
	e3 := &Event{URL: "https://c.com", Title: "C", Source: "extension"}

	require.NoError(t, store.AddEvent(ctx, e1))
	require.NoError(t, store.AddEvent(ctx, e2))
	require.NoError(t, store.AddEvent(ctx, e3))

	results, err := store.SearchEvents(ctx, SearchQuery{Source: "extension", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 2, len(results))
}

func TestSearchEvents_ByTimeRange(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	now := time.Now()

	e1 := &Event{URL: "https://a.com", Title: "Old", Source: "manual", Timestamp: now.Add(-48 * time.Hour)}
	e2 := &Event{URL: "https://b.com", Title: "Recent", Source: "manual", Timestamp: now.Add(-1 * time.Hour)}
	e3 := &Event{URL: "https://c.com", Title: "New", Source: "manual", Timestamp: now}

	require.NoError(t, store.AddEvent(ctx, e1))
	require.NoError(t, store.AddEvent(ctx, e2))
	require.NoError(t, store.AddEvent(ctx, e3))

	// Only events in last 24 hours
	since := now.Add(-24 * time.Hour)
	results, err := store.SearchEvents(ctx, SearchQuery{Since: since, Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 2, len(results), "should find 2 events in last 24 hours")
}

func TestSearchEvents_Pagination(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// Add 5 events
	for i := 0; i < 5; i++ {
		e := &Event{
			URL:    "https://example.com/" + string(rune('a'+i)),
			Title:  "Page " + string(rune('A'+i)),
			Source: "manual",
		}
		require.NoError(t, store.AddEvent(ctx, e))
	}

	// Page 1: limit 2
	page1, err := store.SearchEvents(ctx, SearchQuery{Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, 2, len(page1))

	// Page 2: limit 2, offset 2
	page2, err := store.SearchEvents(ctx, SearchQuery{Limit: 2, Offset: 2})
	require.NoError(t, err)
	assert.Equal(t, 2, len(page2))

	// Ensure no overlap
	assert.NotEqual(t, page1[0].ID, page2[0].ID)
}

func TestSearchEvents_DefaultLimit(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// Add 5 events
	for i := 0; i < 5; i++ {
		e := &Event{
			URL:    "https://example.com/" + string(rune('a'+i)),
			Title:  "Page " + string(rune('A'+i)),
			Source: "manual",
		}
		require.NoError(t, store.AddEvent(ctx, e))
	}

	// No limit specified -- should default to something reasonable
	results, err := store.SearchEvents(ctx, SearchQuery{})
	require.NoError(t, err)
	assert.Equal(t, 5, len(results), "should return all events when under default limit")
}

// --- DeleteEvent ---

func TestDeleteEvent(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	event := &Event{URL: "https://example.com", Title: "Delete Me", Source: "manual"}
	require.NoError(t, store.AddEvent(ctx, event))

	err := store.DeleteEvent(ctx, event.ID)
	require.NoError(t, err)

	// Should not be found
	got, err := store.GetEvent(ctx, event.ID)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestDeleteEvent_CascadesContent(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	event := &Event{URL: "https://example.com", Title: "With Content", Source: "manual"}
	require.NoError(t, store.AddEventWithContent(ctx, event, "Some body text"))

	// Delete the event
	require.NoError(t, store.DeleteEvent(ctx, event.ID))

	// Content should also be gone
	content, err := store.GetContent(ctx, event.ID)
	assert.Error(t, err)
	assert.Nil(t, content)
}

func TestDeleteEvent_NotFound(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	err := store.DeleteEvent(ctx, "CHR-nonexistent")
	assert.Error(t, err)
}

// --- GetContent ---

func TestGetContent_NotFound(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	content, err := store.GetContent(ctx, "CHR-nonexistent")
	assert.Error(t, err)
	assert.Nil(t, content)
}

// --- PruneExpired ---

func TestPruneExpired(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	now := time.Now()

	// Insert old events
	old1 := &Event{URL: "https://old1.com", Title: "Old 1", Source: "manual", Timestamp: now.Add(-72 * time.Hour)}
	old2 := &Event{URL: "https://old2.com", Title: "Old 2", Source: "manual", Timestamp: now.Add(-48 * time.Hour)}
	recent := &Event{URL: "https://recent.com", Title: "Recent", Source: "manual", Timestamp: now}

	require.NoError(t, store.AddEvent(ctx, old1))
	require.NoError(t, store.AddEvent(ctx, old2))
	require.NoError(t, store.AddEvent(ctx, recent))

	// Prune events older than 24 hours
	pruned, err := store.PruneExpired(ctx, now.Add(-24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(2), pruned, "should prune 2 old events")

	// Recent event should still exist
	got, err := store.GetEvent(ctx, recent.ID)
	require.NoError(t, err)
	assert.Equal(t, recent.ID, got.ID)

	// Old events should be gone
	_, err = store.GetEvent(ctx, old1.ID)
	assert.Error(t, err)
}

// --- PurgeAll ---

func TestPurgeAll(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// Add some events with content
	e1 := &Event{URL: "https://a.com", Title: "A", Source: "manual"}
	e2 := &Event{URL: "https://b.com", Title: "B", Source: "manual"}
	require.NoError(t, store.AddEventWithContent(ctx, e1, "Body A"))
	require.NoError(t, store.AddEvent(ctx, e2))

	err := store.PurgeAll(ctx)
	require.NoError(t, err)

	// All events should be gone
	results, err := store.SearchEvents(ctx, SearchQuery{Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, 0, len(results), "should have no events after purge")
}

// --- Exclusions ---

func TestAddEvent_SkipsExcludedDomains(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	// chase.com is in the default exclusions
	event := &Event{
		URL:    "https://chase.com/accounts",
		Title:  "My Bank Account",
		Source: "extension",
	}

	err := store.AddEvent(ctx, event)
	require.NoError(t, err) // Should not error, just skip

	// Event should not have been stored
	assert.Empty(t, event.ID, "excluded event should not get an ID")
}

func TestAddEvent_SkipsRegexExcludedDomains(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	event := &Event{
		URL:    "https://site.xxx/page",
		Title:  "Excluded by regex",
		Source: "extension",
	}

	err := store.AddEvent(ctx, event)
	require.NoError(t, err)
	assert.Empty(t, event.ID, "regex-excluded event should not get an ID")
}

func TestAddEvent_AllowsNonExcludedDomains(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	event := &Event{
		URL:    "https://news.ycombinator.com/item?id=12345",
		Title:  "Hacker News",
		Source: "extension",
	}

	err := store.AddEvent(ctx, event)
	require.NoError(t, err)
	assert.NotEmpty(t, event.ID, "non-excluded event should get an ID")
}

// --- GetStats ---

func TestGetStats_EmptyDB(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	stats, err := store.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.TotalEvents)
	assert.Equal(t, int64(0), stats.TotalContent)
}

func TestGetStats_WithData(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	e1 := &Event{URL: "https://a.com", Title: "A", Source: "manual"}
	e2 := &Event{URL: "https://b.com", Title: "B", Source: "manual"}
	e3 := &Event{URL: "https://a.com/page2", Title: "A2", Source: "extension"}
	require.NoError(t, store.AddEventWithContent(ctx, e1, "Body A"))
	require.NoError(t, store.AddEvent(ctx, e2))
	require.NoError(t, store.AddEventWithContent(ctx, e3, "Body A2"))

	stats, err := store.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.TotalEvents)
	assert.Equal(t, int64(2), stats.TotalContent)
	assert.False(t, stats.OldestEvent.IsZero())
	assert.False(t, stats.NewestEvent.IsZero())
	assert.True(t, len(stats.TopDomains) > 0, "should have top domains")
}

// --- Close ---

func TestClose(t *testing.T) {
	store := openTestStore(t)
	err := store.Close()
	assert.NoError(t, err)
}
