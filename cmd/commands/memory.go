package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/membridge"
	"github.com/dohr-michael/ozzie/internal/models"
	"github.com/dohr-michael/ozzie/pkg/memory"
)

// NewMemoryCommand returns the memory subcommand.
func NewMemoryCommand() *cli.Command {
	return &cli.Command{
		Name:  "memory",
		Usage: "Manage Ozzie's long-term memory",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all memories",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "json", Usage: "Output raw JSON"},
				},
				Action: runMemoryList,
			},
			{
				Name:      "search",
				Usage:     "Search memories",
				ArgsUsage: "<query>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "json", Usage: "Output raw JSON"},
				},
				Action: runMemorySearch,
			},
			{
				Name:      "show",
				Usage:     "Show a memory entry",
				ArgsUsage: "<id>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "json", Usage: "Output raw JSON"},
				},
				Action: runMemoryShow,
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
			{
				Name:   "consolidate",
				Usage:  "Merge similar memories using LLM summarization",
				Action: runMemoryConsolidate,
			},
		},
		DefaultCommand: "list",
	}
}

func newMemoryStore() (*memory.SQLiteStore, error) {
	return memory.NewSQLiteStore(filepath.Join(config.OzziePath(), "memory"))
}

func runMemoryList(_ context.Context, cmd *cli.Command) error {
	store, err := newMemoryStore()
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()

	entries, err := store.List()
	if err != nil {
		return fmt.Errorf("list memories: %w", err)
	}

	if cmd.Bool("json") {
		return json.NewEncoder(os.Stdout).Encode(entries)
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

	store, err := newMemoryStore()
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()

	// Use hybrid retriever if embeddings are enabled
	var vectorStore memory.VectorStorer
	cfg, kr, cfgErr := loadConfigWithKeyRing(cmd.String("config"))
	if cfgErr == nil && cfg.Embedding.IsEnabled() {
		embedder, embErr := membridge.NewEmbedder(ctx, cfg.Embedding, kr)
		if embErr == nil {
			dims := cfg.Embedding.Dims
			if dims <= 0 {
				dims = 1536
			}
			vs, vsErr := memory.NewSQLiteVectorStore(store.DB(), embedder, dims)
			if vsErr == nil {
				vectorStore = vs
			}
		}
	}

	retriever := memory.NewHybridRetriever(store, vectorStore)
	results, err := retriever.Retrieve(ctx, query, nil, 10)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if cmd.Bool("json") {
		return json.NewEncoder(os.Stdout).Encode(results)
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

	store, err := newMemoryStore()
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()

	entry, content, err := store.Get(id)
	if err != nil {
		return fmt.Errorf("get memory: %w", err)
	}

	if cmd.Bool("json") {
		return json.NewEncoder(os.Stdout).Encode(struct {
			Entry   *memory.MemoryEntry `json:"entry"`
			Content string              `json:"content"`
		}{Entry: entry, Content: content})
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

	store, err := newMemoryStore()
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()

	if err := store.Delete(id); err != nil {
		return fmt.Errorf("forget: %w", err)
	}

	fmt.Printf("Memory %s deleted.\n", id)
	return nil
}

func runMemoryReindex(ctx context.Context, cmd *cli.Command) error {
	cfg, kr, err := loadConfigWithKeyRing(cmd.String("config"))
	if err != nil {
		return err
	}

	if !cfg.Embedding.IsEnabled() {
		return fmt.Errorf("embedding is not enabled in config (set embedding.enabled = true)")
	}

	embedder, err := membridge.NewEmbedder(ctx, cfg.Embedding, kr)
	if err != nil {
		return fmt.Errorf("create embedder: %w", err)
	}

	memoryDir := filepath.Join(config.OzziePath(), "memory")
	store, err := memory.NewSQLiteStore(memoryDir)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()

	dims := cfg.Embedding.Dims
	if dims <= 0 {
		dims = 1536
	}
	vectorStore, err := memory.NewSQLiteVectorStore(store.DB(), embedder, dims)
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

func runMemoryConsolidate(ctx context.Context, cmd *cli.Command) error {
	cfg, kr, err := loadConfigWithKeyRing(cmd.String("config"))
	if err != nil {
		return err
	}

	if !cfg.Embedding.IsEnabled() {
		return fmt.Errorf("embedding is not enabled (required for consolidation)")
	}

	memoryDir := filepath.Join(config.OzziePath(), "memory")
	store, err := memory.NewSQLiteStore(memoryDir)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	defer store.Close()

	embedder, err := membridge.NewEmbedder(ctx, cfg.Embedding, kr)
	if err != nil {
		return fmt.Errorf("create embedder: %w", err)
	}

	dims := cfg.Embedding.Dims
	if dims <= 0 {
		dims = 1536
	}
	vectorStore, err := memory.NewSQLiteVectorStore(store.DB(), embedder, dims)
	if err != nil {
		return fmt.Errorf("create vector store: %w", err)
	}

	// Load primary chat model for LLM-based merge
	registry := models.NewRegistry(cfg.Models, kr)
	chatModel, modelErr := registry.Default(ctx)
	if modelErr != nil {
		return fmt.Errorf("load chat model: %w", modelErr)
	}

	consolidator := memory.NewConsolidator(memory.ConsolidatorConfig{
		Store:      store,
		Vector:     vectorStore,
		Summarizer: &extractorLLMAdapter{chatModel: chatModel},
	})

	slog.Info("starting memory consolidation")
	stats, err := consolidator.Run(ctx)
	if err != nil {
		return fmt.Errorf("consolidate: %w", err)
	}

	fmt.Printf("Consolidation complete: checked %d, merged %d groups, %d errors\n",
		stats.Checked, stats.Merged, stats.Errors)
	return nil
}
