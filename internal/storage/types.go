package storage

import "time"

// Event represents a single browsing event captured by Chronicle.
type Event struct {
	ID          string
	URL         string
	Title       string
	Domain      string
	Timestamp   time.Time
	Source      string // "extension", "manual", "import"
	Browser     string
	ContentHash string
	HasBody     bool
	HasEmbed    bool
}

// Content holds the stored body text for an event.
type Content struct {
	EventID     string
	Body        string
	ContentHash string
}

// SearchQuery defines filters for searching events.
type SearchQuery struct {
	Query  string
	Domain string
	Source string
	Since  time.Time
	Until  time.Time
	Limit  int
	Offset int
}

// Stats holds aggregate statistics about the Chronicle database.
type Stats struct {
	TotalEvents       int64
	TotalContent      int64
	OldestEvent       time.Time
	NewestEvent       time.Time
	DatabaseSizeBytes int64
	TopDomains        []DomainCount
}

// DomainCount pairs a domain with its event count.
type DomainCount struct {
	Domain string
	Count  int64
}
