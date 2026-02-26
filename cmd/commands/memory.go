package commands

import (
	"context"
	"fmt"
	"log/slog"
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
			{
				Name:   "reindex",
				Usage:  "Rebuild vector embeddings for all memories",
				Action: runMemoryReindex,
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
	fmt.Fprintln(w, "ID\tTYPE\tTITLE\tTAGS\tIDX\tCONFIDENCE")
	for _, e := range entries {
		tags := "-"
		if len(e.Tags) > 0 {
			tags = fmt.Sprintf("%v", e.Tags)
		}
		idx := "-"
		if e.IsIndexed() {
			idx = "Y"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%.1f\n",
			e.ID, e.Type, e.Title, tags, idx, e.Confidence)
	}
	return w.Flush()
}

func runMemorySearch(ctx context.Context, cmd *cli.Command) error {
	query := cmd.Args().First()
	if query == "" {
		return fmt.Errorf("usage: ozzie memory search <query>")
	}

	store := newMemoryStore()

	// Use hybrid retriever if embeddings are enabled
	var vectorStore *memory.VectorStore
	configPath := cmd.String("config")
	cfg, cfgErr := config.Load(configPath)
	if cfgErr == nil && cfg.Embedding.IsEnabled() {
		embedder, err := memory.NewEmbedder(ctx, cfg.Embedding)
		if err == nil {
			memoryDir := filepath.Join(config.OzziePath(), "memory")
			vs, err := memory.NewVectorStore(ctx, memoryDir, embedder)
			if err == nil {
				vectorStore = vs
			}
		}
	}

	retriever := memory.NewHybridRetriever(store, vectorStore)
	results, err := retriever.Retrieve(query, nil, 10)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No matching memories found.")
		return nil
	}

	mode := "keyword"
	if vectorStore != nil {
		mode = "hybrid"
	}
	fmt.Printf("Search mode: %s\n\n", mode)

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
	if entry.IsIndexed() {
		fmt.Printf("Indexed:    %s (model: %s)\n", entry.IndexedAt.Format("2006-01-02 15:04:05"), entry.EmbeddingModel)
	} else {
		fmt.Printf("Indexed:    no\n")
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

func runMemoryReindex(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !cfg.Embedding.IsEnabled() {
		return fmt.Errorf("embedding is not enabled in config (set embedding.enabled = true)")
	}

	embedder, err := memory.NewEmbedder(ctx, cfg.Embedding)
	if err != nil {
		return fmt.Errorf("create embedder: %w", err)
	}

	memoryDir := filepath.Join(config.OzziePath(), "memory")
	store := memory.NewFileStore(memoryDir)

	vectorStore, err := memory.NewVectorStore(ctx, memoryDir, embedder)
	if err != nil {
		return fmt.Errorf("create vector store: %w", err)
	}

	slog.Info("starting reindex", "driver", cfg.Embedding.Driver, "model", cfg.Embedding.Model)
	stats, err := memory.Reindex(ctx, store, vectorStore, cfg.Embedding.Model)
	if err != nil {
		return fmt.Errorf("reindex: %w", err)
	}

	fmt.Printf("Reindex complete: %d/%d indexed, %d skipped, %d errors\n",
		stats.Indexed, stats.Total, stats.Skipped, stats.Errors)
	return nil
}
