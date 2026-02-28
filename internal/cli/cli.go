package cli

import (
	"fmt"
	"os"

	goflags "github.com/jessevdk/go-flags"
)

// commands holds references to all subcommand structs for inspection/testing.
type commands struct {
	Status *StatusCommand
	Search *SearchCommand
	Open   *OpenCommand
	Add    *AddCommand
	Ingest *IngestCommand
	Prune  *PruneCommand
	Purge  *PurgeCommand
}

// buildParser constructs the go-flags parser with all subcommands registered.
func buildParser(version string) (*goflags.Parser, *GlobalFlags, *commands) {
	var globals GlobalFlags

	parser := goflags.NewParser(&globals, goflags.Default)
	parser.Name = "chronicle"
	parser.LongDescription = "Privacy-first local browsing history capture, search, and recall for fabric."

	cmds := &commands{
		Status: &StatusCommand{globals: &globals, version: version},
		Search: &SearchCommand{globals: &globals, version: version},
		Open:   &OpenCommand{globals: &globals, version: version},
		Add:    &AddCommand{globals: &globals, version: version},
		Ingest: &IngestCommand{globals: &globals, version: version},
		Prune:  &PruneCommand{globals: &globals, version: version},
		Purge:  &PurgeCommand{globals: &globals, version: version},
	}

	parser.AddCommand("status", "Show ingestion health and statistics", "Show ingestion health, database statistics, and configuration summary.", cmds.Status)
	parser.AddCommand("search", "Search captured events", "Search captured events by keyword, with optional filters.", cmds.Search)
	parser.AddCommand("open", "Print stored content of an event", "Print the full stored content of a specific event.", cmds.Open)
	parser.AddCommand("add", "Manually ingest a URL/title/body", "Manually ingest a URL/title/body into Chronicle.", cmds.Add)
	parser.AddCommand("ingest", "Start the Chronicle daemon", "Start the Chronicle daemon (local HTTP service).", cmds.Ingest)
	parser.AddCommand("prune", "Apply TTL pruning", "Apply TTL pruning to remove old events.", cmds.Prune)
	parser.AddCommand("purge", "Delete ALL Chronicle data", "Delete ALL Chronicle data. Destructive operation with safety prompt.", cmds.Purge)

	return parser, &globals, cmds
}

// Run is the main entry point for the Chronicle CLI using os.Args.
func Run(version string) error {
	return RunWithArgs(version, nil)
}

// RunWithArgs parses the given args (or os.Args if nil) and executes the matched subcommand.
func RunWithArgs(version string, args []string) error {
	// Handle --version before parser (go-flags requires a subcommand, but
	// --version is valid without one).
	checkArgs := args
	if checkArgs == nil {
		checkArgs = os.Args[1:]
	}
	for _, arg := range checkArgs {
		if arg == "--version" {
			fmt.Printf("chronicle %s\n", version)
			return nil
		}
		if arg == "--" {
			break
		}
	}

	parser, _, _ := buildParser(version)

	var err error
	if args != nil {
		_, err = parser.ParseArgs(args)
	} else {
		_, err = parser.Parse()
	}

	if err != nil {
		if flagsErr, ok := err.(*goflags.Error); ok {
			if flagsErr.Type == goflags.ErrHelp {
				return nil
			}
		}
		return err
	}

	return nil
}
