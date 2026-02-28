package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// pruneJSON is the JSON output structure for the prune command.
type pruneJSON struct {
	Pruned   int64  `json:"pruned"`
	OlderThan string `json:"older_than"`
	DryRun   bool   `json:"dry_run"`
}

// Execute implements the go-flags Commander interface for PruneCommand.
func (c *PruneCommand) Execute(args []string) error {
	// Determine the retention duration.
	var retention time.Duration
	var olderThanLabel string

	if c.OlderThan != "" {
		d, err := parseDuration(c.OlderThan)
		if err != nil {
			return fmt.Errorf("invalid duration for --older-than: %w", err)
		}
		retention = d
		olderThanLabel = c.OlderThan
	} else {
		retention = time.Duration(defaultRetentionDays) * 24 * time.Hour
		olderThanLabel = fmt.Sprintf("%dd", defaultRetentionDays)
	}

	cutoff := time.Now().Add(-retention)
	humanDur := formatDurationHuman(retention)

	// Open store (use injected store for tests, default DB otherwise).
	store := c.store
	if store == nil {
		s, db, err := openDefaultStore()
		if err != nil {
			return err
		}
		defer db.Close()
		defer s.Close()
		store = s
	}

	ctx := context.Background()

	// Count events that would be pruned.
	count, err := store.CountExpired(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("count expired events: %w", err)
	}

	// Nothing to prune.
	if count == 0 {
		if c.globals != nil && c.globals.JSON {
			return json.NewEncoder(os.Stdout).Encode(pruneJSON{
				Pruned:    0,
				OlderThan: olderThanLabel,
				DryRun:    c.DryRun,
			})
		}
		fmt.Printf("No events to prune (older than %s).\n", humanDur)
		return nil
	}

	// Dry run: report and exit.
	if c.DryRun {
		if c.globals != nil && c.globals.JSON {
			return json.NewEncoder(os.Stdout).Encode(pruneJSON{
				Pruned:    count,
				OlderThan: olderThanLabel,
				DryRun:    true,
			})
		}
		fmt.Printf("[DRY RUN] Would prune %d events older than %s.\n", count, humanDur)
		return nil
	}

	// Confirmation prompt (unless --force).
	if !c.Force {
		fmt.Printf("Pruning events older than %s...\n", humanDur)
		fmt.Printf("Found %d events to prune.\n", count)
		fmt.Print("Proceed? [y/N] ")

		reader := c.stdin
		if reader == nil {
			reader = os.Stdin
		}
		scanner := bufio.NewScanner(reader)
		scanner.Scan()
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Execute prune.
	pruned, err := store.PruneExpired(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("prune failed: %w", err)
	}

	if c.globals != nil && c.globals.JSON {
		return json.NewEncoder(os.Stdout).Encode(pruneJSON{
			Pruned:    pruned,
			OlderThan: olderThanLabel,
			DryRun:    false,
		})
	}

	fmt.Printf("Pruned %d events older than %s.\n", pruned, humanDur)
	return nil
}
