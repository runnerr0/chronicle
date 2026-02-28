package cli

import "database/sql"

// GlobalFlags holds flags available to all subcommands.
type GlobalFlags struct {
	Config  string `long:"config" description:"Path to config file" default:""`
	JSON    bool   `long:"json" description:"Output in JSON format"`
	Verbose bool   `long:"verbose" description:"Enable verbose output"`
	Version bool   `long:"version" description:"Show version and exit"`
}

// StatusCommand — show ingestion health, database stats, config summary.
type StatusCommand struct {
	globals *GlobalFlags
	version string
}

// SearchCommand — search captured events by keyword with filters.
type SearchCommand struct {
	Since        string   `long:"since" description:"Only events newer than duration (e.g., 7d, 24h, 2w)" default:"30d"`
	Until        string   `long:"until" description:"Only events older than duration"`
	Domain       []string `long:"domain" description:"Filter by domain (repeatable)"`
	Browser      []string `long:"browser" description:"Filter by browser (repeatable)"`
	HasBody      bool     `long:"has-body" description:"Only events with captured body content"`
	HasEmbedding bool     `long:"has-embedding" description:"Only events with generated embeddings"`
	Semantic     bool     `long:"semantic" description:"Use semantic search (requires embeddings enabled)"`
	Hybrid       bool     `long:"hybrid" description:"Use hybrid search: keyword + semantic"`
	Limit        int      `long:"limit" description:"Maximum results" default:"10"`
	Offset       int      `long:"offset" description:"Skip first N results" default:"0"`

	globals *GlobalFlags
	version string
}

// OpenCommand — print the full stored content of a specific event.
type OpenCommand struct {
	ID     string `long:"id" description:"Event ID (required)"`
	Format string `long:"format" description:"Output format: raw | md | json" default:"md"`

	globals *GlobalFlags
	version string
}

// AddCommand — manually ingest a URL/title/body into Chronicle.
type AddCommand struct {
	URL         string `long:"url" description:"URL to record (required)"`
	Title       string `long:"title" description:"Page title (required)"`
	BodyFile    string `long:"body-file" description:"Path to file containing body content"`
	Body        string `long:"body" description:"Inline body text"`
	BrowserName string `long:"browser" description:"Source browser label" default:"manual"`
	Embed       bool   `long:"embed" description:"Generate embedding immediately"`

	globals *GlobalFlags
	version string
}

// IngestCommand — start the Chronicle daemon (local HTTP service).
type IngestCommand struct {
	Foreground bool   `long:"foreground" description:"Run in foreground (don't daemonize)"`
	Port       int    `long:"port" description:"Override daemon port"`
	LogLevel   string `long:"log-level" description:"Override log level"`

	globals *GlobalFlags
	version string
}

// PruneCommand — apply TTL pruning to remove old events.
type PruneCommand struct {
	Now       bool   `long:"now" description:"Prune immediately"`
	OlderThan string `long:"older-than" description:"Override retention period (e.g., 30d)"`
	DryRun    bool   `long:"dry-run" description:"Show what would be pruned without deleting"`

	globals *GlobalFlags
	version string
}

// PurgeCommand — delete ALL Chronicle data with safety confirmation.
type PurgeCommand struct {
	All   bool `long:"all" description:"Required flag to confirm purge intent"`
	Force bool `long:"force" description:"Skip safety confirmation prompt"`

	globals *GlobalFlags
	version string
	db      *sql.DB // injectable for testing; nil means open default DB
}
