package cli

import "fmt"

// Execute implements the go-flags Commander interface for SearchCommand.
func (c *SearchCommand) Execute(args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	// TODO: Implement full search (P0-7)
	fmt.Printf("Searching for: %q (since %s, limit %d)\n", query, c.Since, c.Limit)
	fmt.Println("No results found.")
	return nil
}
