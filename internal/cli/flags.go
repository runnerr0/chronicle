package cli

// Flags defines all CLI flags for chronicle using go-flags.
// This mirrors fabric's go-flags approach for ecosystem consistency.
type Flags struct {
	// Global flags
	Config  string `long:"config" description:"Path to config file" default:""`
	JSON    bool   `long:"json" description:"Output in JSON format"`
	Verbose bool   `long:"verbose" description:"Enable verbose output"`
	Version bool   `long:"version" description:"Show version and exit"`

	// Command flags — exactly one command should be active
	Status bool `long:"status" description:"Show ingestion health and statistics"`
	Search bool `long:"search" description:"Search captured events"`
	Open   bool `long:"open" description:"Print stored content of a specific event"`
	Add    bool `long:"add" description:"Manually ingest a URL/title/body"`
	Ingest bool `long:"ingest" description:"Start the Chronicle daemon"`
	Prune  bool `long:"prune" description:"Apply TTL pruning"`
	Purge  bool `long:"purge" description:"Delete ALL Chronicle data"`

	// Search flags
	Query  string `short:"q" long:"query" description:"Search query"`
	Since  string `long:"since" description:"Only events newer than duration (e.g., 7d, 24h)" default:"30d"`
	Until  string `long:"until" description:"Only events older than duration"`
	Domain string `long:"domain" description:"Filter by domain"`
	Browser string `long:"browser" description:"Filter by browser"`
	HasBody bool  `long:"has-body" description:"Only events with captured body"`
	HasEmbed bool `long:"has-embedding" description:"Only events with embeddings"`
	Semantic bool `long:"semantic" description:"Use semantic search"`
	Hybrid  bool  `long:"hybrid" description:"Use hybrid search (keyword + semantic)"`
	Limit   int   `long:"limit" description:"Maximum results" default:"10"`
	Offset  int   `long:"offset" description:"Skip first N results" default:"0"`

	// Open flags
	ID     string `long:"id" description:"Event ID"`
	Format string `long:"format" description:"Output format: raw, md, json" default:"md"`

	// Add flags
	URL      string `long:"url" description:"URL to record"`
	Title    string `long:"title" description:"Page title"`
	BodyFile string `long:"body-file" description:"Path to file containing body content"`
	Body     string `long:"body" description:"Inline body text"`
	BrowserName string `long:"browser-name" description:"Source browser label" default:"manual"`
	Embed    bool   `long:"embed" description:"Generate embedding immediately"`

	// Ingest flags
	Foreground bool `long:"foreground" description:"Run daemon in foreground"`
	Port       int  `long:"port" description:"Override daemon port"`
	LogLevel   string `long:"log-level" description:"Override log level"`

	// Prune flags
	Now      bool   `long:"now" description:"Prune immediately"`
	OlderThan string `long:"older-than" description:"Override retention period (e.g., 30d)"`
	DryRun   bool   `long:"dry-run" description:"Show what would be pruned without deleting"`

	// Purge flags
	All   bool `long:"all" description:"Required to confirm purge intent"`
	Force bool `long:"force" description:"Skip safety confirmation prompt"`
}
