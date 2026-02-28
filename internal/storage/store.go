package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Store defines the interface for Chronicle data operations.
type Store interface {
	AddEvent(ctx context.Context, event *Event) error
	AddEventWithContent(ctx context.Context, event *Event, body string) error
	GetEvent(ctx context.Context, id string) (*Event, error)
	SearchEvents(ctx context.Context, query SearchQuery) ([]Event, error)
	DeleteEvent(ctx context.Context, id string) error
	GetContent(ctx context.Context, eventID string) (*Content, error)
	PruneExpired(ctx context.Context, olderThan time.Time) (int64, error)
	PurgeAll(ctx context.Context) error
	GetStats(ctx context.Context) (*Stats, error)
	Close() error
}

// SQLiteStore implements Store backed by a SQLite database.
type SQLiteStore struct {
	db *sql.DB

	// Prepared statements
	insertEvent   *sql.Stmt
	insertContent *sql.Stmt
	getEvent      *sql.Stmt
	deleteEvent   *sql.Stmt
	getContent    *sql.Stmt

	// Cached exclusion rules (loaded once at init)
	domainExclusions []string
	regexExclusions  []*regexp.Regexp
}

// NewSQLiteStore creates a new SQLiteStore from an already-opened and migrated database.
func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	s := &SQLiteStore{db: db}

	if err := s.prepareStatements(); err != nil {
		return nil, fmt.Errorf("prepare statements: %w", err)
	}

	if err := s.initFTS(); err != nil {
		return nil, fmt.Errorf("init FTS: %w", err)
	}

	if err := s.loadExclusions(); err != nil {
		return nil, fmt.Errorf("load exclusions: %w", err)
	}

	return s, nil
}

func (s *SQLiteStore) prepareStatements() error {
	var err error

	s.insertEvent, err = s.db.Prepare(`
		INSERT INTO events (id, ts, url, title, domain, browser, source, has_body, has_embedding, content_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}

	s.insertContent, err = s.db.Prepare(`
		INSERT INTO content (event_id, body, byte_size)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return err
	}

	s.getEvent, err = s.db.Prepare(`
		SELECT id, ts, url, title, domain, browser, source, has_body, has_embedding, content_hash
		FROM events WHERE id = ?
	`)
	if err != nil {
		return err
	}

	s.deleteEvent, err = s.db.Prepare(`DELETE FROM events WHERE id = ?`)
	if err != nil {
		return err
	}

	s.getContent, err = s.db.Prepare(`
		SELECT event_id, body FROM content WHERE event_id = ?
	`)
	if err != nil {
		return err
	}

	return nil
}

// initFTS creates the FTS5 virtual table for full-text search if it doesn't exist.
func (s *SQLiteStore) initFTS() error {
	_, err := s.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS events_fts USING fts5(
			event_id UNINDEXED,
			title,
			url,
			tokenize='unicode61'
		)
	`)
	return err
}

// loadExclusions loads domain and regex exclusion rules from the database.
func (s *SQLiteStore) loadExclusions() error {
	rows, err := s.db.Query("SELECT rule_type, rule_value FROM exclusions")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var ruleType, ruleValue string
		if err := rows.Scan(&ruleType, &ruleValue); err != nil {
			return err
		}
		switch ruleType {
		case "domain":
			s.domainExclusions = append(s.domainExclusions, ruleValue)
		case "regex":
			re, err := regexp.Compile(ruleValue)
			if err != nil {
				continue // skip invalid regex
			}
			s.regexExclusions = append(s.regexExclusions, re)
		}
	}

	return rows.Err()
}

// isExcluded checks if a domain is blocked by exclusion rules.
func (s *SQLiteStore) isExcluded(domain string) bool {
	for _, d := range s.domainExclusions {
		if d == domain {
			return true
		}
	}
	for _, re := range s.regexExclusions {
		if re.MatchString(domain) {
			return true
		}
	}
	return false
}

// generateID creates a Chronicle event ID: CHR- + 8 random hex chars.
func generateID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "CHR-" + hex.EncodeToString(b), nil
}

// ftsQuery converts a user search string into a valid FTS5 query.
// Each word becomes a quoted prefix token joined with OR.
func ftsQuery(input string) string {
	words := strings.Fields(input)
	if len(words) == 0 {
		return ""
	}
	var parts []string
	for _, w := range words {
		// Quote each term, add prefix wildcard for partial matching
		parts = append(parts, `"`+w+`"*`)
	}
	return strings.Join(parts, " OR ")
}

// parseTimestamp tries several common SQLite timestamp formats.
func parseTimestamp(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999-07:00",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp: %s", s)
}

// extractDomain pulls the hostname from a URL string.
func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// AddEvent inserts a new event into the database. The event's ID and Domain
// fields are populated automatically. If the domain is excluded, the event
// is silently skipped (ID remains empty, no error).
func (s *SQLiteStore) AddEvent(ctx context.Context, event *Event) error {
	event.Domain = extractDomain(event.URL)

	if s.isExcluded(event.Domain) {
		return nil // silently skip
	}

	id, err := generateID()
	if err != nil {
		return fmt.Errorf("generate ID: %w", err)
	}
	event.ID = id

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	tsFormatted := event.Timestamp.UTC().Format(time.RFC3339)
	_, err = s.insertEvent.ExecContext(ctx,
		event.ID, tsFormatted, event.URL, event.Title, event.Domain,
		event.Browser, event.Source, event.HasBody, event.HasEmbed, event.ContentHash,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	// Index in FTS
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO events_fts (event_id, title, url) VALUES (?, ?, ?)",
		event.ID, event.Title, event.URL,
	)
	if err != nil {
		return fmt.Errorf("insert FTS: %w", err)
	}

	return nil
}

// AddEventWithContent inserts an event and its body content in a single transaction.
func (s *SQLiteStore) AddEventWithContent(ctx context.Context, event *Event, body string) error {
	event.Domain = extractDomain(event.URL)

	if s.isExcluded(event.Domain) {
		return nil
	}

	id, err := generateID()
	if err != nil {
		return fmt.Errorf("generate ID: %w", err)
	}
	event.ID = id
	event.HasBody = true

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	tsFormatted := event.Timestamp.UTC().Format(time.RFC3339)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (id, ts, url, title, domain, browser, source, has_body, has_embedding, content_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, tsFormatted, event.URL, event.Title, event.Domain,
		event.Browser, event.Source, true, event.HasEmbed, event.ContentHash,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO content (event_id, body, byte_size) VALUES (?, ?, ?)",
		event.ID, body, len(body),
	)
	if err != nil {
		return fmt.Errorf("insert content: %w", err)
	}

	// FTS index with body included
	_, err = tx.ExecContext(ctx,
		"INSERT INTO events_fts (event_id, title, url) VALUES (?, ?, ?)",
		event.ID, event.Title, event.URL,
	)
	if err != nil {
		return fmt.Errorf("insert FTS: %w", err)
	}

	return tx.Commit()
}

// GetEvent retrieves a single event by ID.
func (s *SQLiteStore) GetEvent(ctx context.Context, id string) (*Event, error) {
	var e Event
	var contentHash sql.NullString
	var tsStr string

	err := s.getEvent.QueryRowContext(ctx, id).Scan(
		&e.ID, &tsStr, &e.URL, &e.Title, &e.Domain,
		&e.Browser, &e.Source, &e.HasBody, &e.HasEmbed, &contentHash,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("event %s not found", id)
		}
		return nil, fmt.Errorf("get event: %w", err)
	}

	e.Timestamp, _ = parseTimestamp(tsStr)

	if contentHash.Valid {
		e.ContentHash = contentHash.String
	}

	return &e, nil
}

// SearchEvents queries events with optional filters.
func (s *SQLiteStore) SearchEvents(ctx context.Context, q SearchQuery) ([]Event, error) {
	if q.Limit <= 0 {
		q.Limit = 50
	}

	// If there's a text query, use FTS
	if q.Query != "" {
		return s.searchFTS(ctx, q)
	}

	return s.searchFiltered(ctx, q)
}

// searchFTS uses the FTS5 index for keyword search, then joins with events table for filtering.
func (s *SQLiteStore) searchFTS(ctx context.Context, q SearchQuery) ([]Event, error) {
	var clauses []string
	var args []interface{}

	baseQuery := `
		SELECT e.id, e.ts, e.url, e.title, e.domain, e.browser, e.source,
		       e.has_body, e.has_embedding, e.content_hash
		FROM events_fts f
		JOIN events e ON e.id = f.event_id
	`

	// Quote each word for FTS5 prefix matching
	clauses = append(clauses, "events_fts MATCH ?")
	args = append(args, ftsQuery(q.Query))

	if q.Domain != "" {
		clauses = append(clauses, "e.domain = ?")
		args = append(args, q.Domain)
	}
	if q.Source != "" {
		clauses = append(clauses, "e.source = ?")
		args = append(args, q.Source)
	}
	if !q.Since.IsZero() {
		clauses = append(clauses, "e.ts >= ?")
		args = append(args, q.Since.UTC().Format(time.RFC3339))
	}
	if !q.Until.IsZero() {
		clauses = append(clauses, "e.ts <= ?")
		args = append(args, q.Until.UTC().Format(time.RFC3339))
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	fullQuery := baseQuery + where + " ORDER BY rank LIMIT ? OFFSET ?"
	args = append(args, q.Limit, q.Offset)

	return s.scanEvents(ctx, fullQuery, args...)
}

// searchFiltered queries events using standard SQL filters (no FTS).
func (s *SQLiteStore) searchFiltered(ctx context.Context, q SearchQuery) ([]Event, error) {
	var clauses []string
	var args []interface{}

	baseQuery := `
		SELECT id, ts, url, title, domain, browser, source,
		       has_body, has_embedding, content_hash
		FROM events
	`

	if q.Domain != "" {
		clauses = append(clauses, "domain = ?")
		args = append(args, q.Domain)
	}
	if q.Source != "" {
		clauses = append(clauses, "source = ?")
		args = append(args, q.Source)
	}
	if !q.Since.IsZero() {
		clauses = append(clauses, "ts >= ?")
		args = append(args, q.Since.UTC().Format(time.RFC3339))
	}
	if !q.Until.IsZero() {
		clauses = append(clauses, "ts <= ?")
		args = append(args, q.Until.UTC().Format(time.RFC3339))
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	fullQuery := baseQuery + where + " ORDER BY ts DESC LIMIT ? OFFSET ?"
	args = append(args, q.Limit, q.Offset)

	return s.scanEvents(ctx, fullQuery, args...)
}

// scanEvents executes a query and scans results into Event slices.
func (s *SQLiteStore) scanEvents(ctx context.Context, query string, args ...interface{}) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var contentHash sql.NullString
		var tsStr string
		if err := rows.Scan(
			&e.ID, &tsStr, &e.URL, &e.Title, &e.Domain,
			&e.Browser, &e.Source, &e.HasBody, &e.HasEmbed, &contentHash,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.Timestamp, _ = parseTimestamp(tsStr)
		if contentHash.Valid {
			e.ContentHash = contentHash.String
		}
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Return empty slice rather than nil
	if events == nil {
		events = []Event{}
	}

	return events, nil
}

// DeleteEvent removes an event by ID. Content is cascade-deleted by the schema.
func (s *SQLiteStore) DeleteEvent(ctx context.Context, id string) error {
	// Also clean up FTS
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM events_fts WHERE event_id = ?", id,
	)
	if err != nil {
		return fmt.Errorf("delete FTS entry: %w", err)
	}

	res, err := s.deleteEvent.ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("event %s not found", id)
	}

	return nil
}

// GetContent retrieves the stored body for an event.
func (s *SQLiteStore) GetContent(ctx context.Context, eventID string) (*Content, error) {
	var c Content
	err := s.getContent.QueryRowContext(ctx, eventID).Scan(&c.EventID, &c.Body)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("content for event %s not found", eventID)
		}
		return nil, fmt.Errorf("get content: %w", err)
	}
	return &c, nil
}

// PruneExpired deletes events with timestamps before olderThan.
func (s *SQLiteStore) PruneExpired(ctx context.Context, olderThan time.Time) (int64, error) {
	tsFormatted := olderThan.UTC().Format(time.RFC3339)

	// Clean FTS entries first
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM events_fts WHERE event_id IN (
			SELECT id FROM events WHERE ts < ?
		)`, tsFormatted,
	)
	if err != nil {
		return 0, fmt.Errorf("prune FTS: %w", err)
	}

	res, err := s.db.ExecContext(ctx, "DELETE FROM events WHERE ts < ?", tsFormatted)
	if err != nil {
		return 0, fmt.Errorf("prune events: %w", err)
	}

	return res.RowsAffected()
}

// PurgeAll deletes all events and content.
func (s *SQLiteStore) PurgeAll(ctx context.Context) error {
	stmts := []string{
		"DROP TABLE IF EXISTS events_fts",
		"DELETE FROM content",
		"DELETE FROM events",
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("purge (%s): %w", stmt, err)
		}
	}
	// Recreate FTS table
	return s.initFTS()
}

// GetStats returns aggregate statistics about the database.
func (s *SQLiteStore) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{}

	// Total events
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&stats.TotalEvents)
	if err != nil {
		return nil, fmt.Errorf("count events: %w", err)
	}

	// Total content
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM content").Scan(&stats.TotalContent)
	if err != nil {
		return nil, fmt.Errorf("count content: %w", err)
	}

	// Oldest and newest (handle empty DB)
	if stats.TotalEvents > 0 {
		var oldestStr, newestStr string
		err = s.db.QueryRowContext(ctx, "SELECT MIN(ts), MAX(ts) FROM events").Scan(&oldestStr, &newestStr)
		if err != nil {
			return nil, fmt.Errorf("event time range: %w", err)
		}
		stats.OldestEvent, _ = parseTimestamp(oldestStr)
		stats.NewestEvent, _ = parseTimestamp(newestStr)
	}

	// Top domains
	rows, err := s.db.QueryContext(ctx,
		"SELECT domain, COUNT(*) as cnt FROM events GROUP BY domain ORDER BY cnt DESC LIMIT 10",
	)
	if err != nil {
		return nil, fmt.Errorf("top domains: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var dc DomainCount
		if err := rows.Scan(&dc.Domain, &dc.Count); err != nil {
			return nil, err
		}
		stats.TopDomains = append(stats.TopDomains, dc)
	}

	return stats, rows.Err()
}

// Close releases all prepared statements. The underlying *sql.DB is NOT
// closed — that is the caller's responsibility.
func (s *SQLiteStore) Close() error {
	stmts := []*sql.Stmt{
		s.insertEvent, s.insertContent, s.getEvent,
		s.deleteEvent, s.getContent,
	}
	for _, stmt := range stmts {
		if stmt != nil {
			stmt.Close()
		}
	}
	return nil
}
