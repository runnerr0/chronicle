package cli

import "fmt"

func handleSearch(flags *Flags) (bool, error) {
	if !flags.Search {
		return false, nil
	}

	// TODO: Implement full search (P0-7)
	fmt.Printf("Searching for: %q (since %s, limit %d)\n", flags.Query, flags.Since, flags.Limit)
	fmt.Println("No results found.")
	return true, nil
}
