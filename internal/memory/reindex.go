package memory

import (
	"context"
	"fmt"
	"log/slog"
)

// ReindexStats holds the results of a reindex operation.
type ReindexStats struct {
	Total   int
	Indexed int
	Errors  int
}

// Reindex rebuilds vector embeddings for all memories in the store.
// It respects context cancellation and logs progress every 10 items.
func Reindex(ctx context.Context, store Store, vector *VectorStore) (*ReindexStats, error) {
	entries, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("reindex: list memories: %w", err)
	}

	stats := &ReindexStats{Total: len(entries)}
	for i, entry := range entries {
		if err := ctx.Err(); err != nil {
			return stats, fmt.Errorf("reindex: cancelled after %d/%d: %w", stats.Indexed, stats.Total, err)
		}

		_, content, err := store.Get(entry.ID)
		if err != nil {
			slog.Warn("reindex: failed to read content", "id", entry.ID, "error", err)
			stats.Errors++
			continue
		}

		text := BuildEmbedText(entry, content)
		meta := BuildEmbedMeta(entry)

		if err := vector.Upsert(ctx, entry.ID, text, meta); err != nil {
			slog.Warn("reindex: failed to upsert", "id", entry.ID, "error", err)
			stats.Errors++
			continue
		}

		stats.Indexed++
		if (i+1)%10 == 0 {
			slog.Info("reindex progress", "indexed", stats.Indexed, "total", stats.Total)
		}
	}

	slog.Info("reindex complete", "indexed", stats.Indexed, "errors", stats.Errors, "total", stats.Total)
	return stats, nil
}
