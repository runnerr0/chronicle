package cli

import "fmt"

// Execute implements the go-flags Commander interface for OpenCommand.
func (c *OpenCommand) Execute(args []string) error {
	if c.ID == "" {
		return fmt.Errorf("--id is required for open command")
	}

	// TODO: Implement full open (P0-8)
	fmt.Printf("Event %s not found.\n", c.ID)
	return nil
}
