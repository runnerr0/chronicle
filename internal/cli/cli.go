package cli

import (
	"fmt"

	goflags "github.com/jessevdk/go-flags"
)

// Run is the main entry point for the Chronicle CLI.
// It parses flags and routes to the appropriate handler.
func Run(version string) error {
	var flags Flags

	parser := goflags.NewParser(&flags, goflags.Default)
	parser.Name = "chronicle"
	parser.LongDescription = "Privacy-first local browsing history capture, search, and recall for fabric."

	_, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*goflags.Error); ok && flagsErr.Type == goflags.ErrHelp {
			return nil
		}
		return err
	}

	// Version flag
	if flags.Version {
		fmt.Printf("chronicle %s\n", version)
		return nil
	}

	// Sequential handler chain (matching fabric's pattern).
	// Each handler returns (handled, error). Processing stops on first match.
	handlers := []func(*Flags) (bool, error){
		handleStatus,
		handleSearch,
		handleOpen,
		handleAdd,
		handleIngest,
		handlePrune,
		handlePurge,
	}

	for _, handler := range handlers {
		handled, err := handler(&flags)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
	}

	// No command matched — show help
	parser.WriteHelp(fmt.Printf)
	return nil
}
