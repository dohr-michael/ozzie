package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/memory"
)

// NewMemoryCommand returns the memory subcommand.
func NewMemoryCommand() *cli.Command {
	return &cli.Command{
		Name:  "memory",
		Usage: "Manage Ozzie's long-term memory",
		Commands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List all memories",
				Action: runMemoryList,
			},
			{
				Name:      "search",
				Usage:     "Search memories",
				ArgsUsage: "<query>",
				Action:    runMemorySearch,
			},
			{
				Name:      "show",
				Usage:     "Show a memory entry",
				ArgsUsage: "<id>",
				Action:    runMemoryShow,
			},
			{
				Name:      "forget",
				Usage:     "Delete a memory",
				ArgsUsage: "<id>",
				Action:    runMemoryForget,
			},
		},
		DefaultCommand: "list",
	}
}

func newMemoryStore() *memory.FileStore {
	return memory.NewFileStore(filepath.Join(config.OzziePath(), "memory"))
}

func runMemoryList(_ context.Context, _ *cli.Command) error {
	store := newMemoryStore()

	entries, err := store.List()
	if err != nil {
		return fmt.Errorf("list memories: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No memories stored.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tTITLE\tTAGS\tCONFIDENCE")
	for _, e := range entries {
		tags := "-"
		if len(e.Tags) > 0 {
			tags = fmt.Sprintf("%v", e.Tags)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.1f\n",
			e.ID, e.Type, e.Title, tags, e.Confidence)
	}
	return w.Flush()
}

func runMemorySearch(_ context.Context, cmd *cli.Command) error {
	query := cmd.Args().First()
	if query == "" {
		return fmt.Errorf("usage: ozzie memory search <query>")
	}

	store := newMemoryStore()
	retriever := memory.NewRetriever(store)

	results, err := retriever.Retrieve(query, nil, 10)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No matching memories found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SCORE\tID\tTYPE\tTITLE")
	for _, r := range results {
		fmt.Fprintf(w, "%.2f\t%s\t%s\t%s\n",
			r.Score, r.Entry.ID, r.Entry.Type, r.Entry.Title)
	}
	return w.Flush()
}

func runMemoryShow(_ context.Context, cmd *cli.Command) error {
	id := cmd.Args().First()
	if id == "" {
		return fmt.Errorf("usage: ozzie memory show <id>")
	}

	store := newMemoryStore()

	entry, content, err := store.Get(id)
	if err != nil {
		return fmt.Errorf("get memory: %w", err)
	}

	fmt.Printf("ID:         %s\n", entry.ID)
	fmt.Printf("Title:      %s\n", entry.Title)
	fmt.Printf("Type:       %s\n", entry.Type)
	fmt.Printf("Source:     %s\n", entry.Source)
	fmt.Printf("Confidence: %.2f\n", entry.Confidence)
	fmt.Printf("Created:    %s\n", entry.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:    %s\n", entry.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Last used:  %s\n", entry.LastUsedAt.Format("2006-01-02 15:04:05"))
	if len(entry.Tags) > 0 {
		fmt.Printf("Tags:       %v\n", entry.Tags)
	}
	fmt.Printf("\nContent:\n%s\n", content)

	return nil
}

func runMemoryForget(_ context.Context, cmd *cli.Command) error {
	id := cmd.Args().First()
	if id == "" {
		return fmt.Errorf("usage: ozzie memory forget <id>")
	}

	store := newMemoryStore()

	if err := store.Delete(id); err != nil {
		return fmt.Errorf("forget: %w", err)
	}

	fmt.Printf("Memory %s deleted.\n", id)
	return nil
}
