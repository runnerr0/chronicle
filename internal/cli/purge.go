package cli

import "fmt"

// Execute implements the go-flags Commander interface for PurgeCommand.
func (c *PurgeCommand) Execute(args []string) error {
	if !c.All {
		return fmt.Errorf("--all flag is required to confirm purge intent")
	}

	// TODO: Implement full purge (P0-10)
	fmt.Println("Purging... (stub)")
	return nil
}
