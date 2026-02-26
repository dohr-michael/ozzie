package memory

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ReindexStats holds the results of a reindex operation.
type ReindexStats struct {
	Total   int
	Indexed int
	Skipped int
	Errors  int
}

// Reindex rebuilds vector embeddings for all memories in the store.
// It skips entries that are already indexed with the same model and not stale.
// It respects context cancellation and logs progress every 10 items.
func Reindex(ctx context.Context, store Store, vector *VectorStore, modelName string) (*ReindexStats, error) {
	entries, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("reindex: list memories: %w", err)
	}

	stats := &ReindexStats{Total: len(entries)}
	for i, entry := range entries {
		if err := ctx.Err(); err != nil {
			return stats, fmt.Errorf("reindex: cancelled after %d/%d: %w", stats.Indexed, stats.Total, err)
		}

		// Skip entries already indexed with the same model and not stale
		if !entry.IsStale(modelName) {
			stats.Skipped++
			continue
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

		// Mark entry as indexed
		now := time.Now()
		entry.EmbeddingModel = modelName
		entry.IndexedAt = &now
		if err := store.Update(entry, content); err != nil {
			slog.Warn("reindex: failed to mark indexed", "id", entry.ID, "error", err)
		}

		stats.Indexed++
		if (i+1)%10 == 0 {
			slog.Info("reindex progress", "indexed", stats.Indexed, "skipped", stats.Skipped, "total", stats.Total)
		}
	}

	slog.Info("reindex complete", "indexed", stats.Indexed, "skipped", stats.Skipped, "errors", stats.Errors, "total", stats.Total)
	return stats, nil
}
