package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// LLMSummarizer generates a text summary from a prompt.
type LLMSummarizer interface {
	Summarize(ctx context.Context, prompt string) (string, error)
}

const (
	defaultSimilarityThreshold = 0.85
	consolidationPrompt        = `You are merging similar memories into a single consolidated entry.

Source memories:
%s

Create a single merged memory that combines all the information.
Respond with JSON:
{
  "title": "concise merged title",
  "content": "complete merged content in markdown",
  "tags": ["merged", "tag1", "tag2"],
  "type": "preference|fact|procedure|context"
}`
)

// Consolidator merges similar memories using LLM summarization.
type Consolidator struct {
	store      *SQLiteStore
	vector     VectorStorer
	summarizer LLMSummarizer
	threshold  float64 // cosine similarity threshold for merge candidates
}

// ConsolidatorConfig holds configuration for the Consolidator.
type ConsolidatorConfig struct {
	Store      *SQLiteStore
	Vector     VectorStorer
	Summarizer LLMSummarizer
	Threshold  float64 // default: 0.85
}

// NewConsolidator creates a new Consolidator.
func NewConsolidator(cfg ConsolidatorConfig) *Consolidator {
	threshold := cfg.Threshold
	if threshold <= 0 {
		threshold = defaultSimilarityThreshold
	}
	return &Consolidator{
		store:      cfg.Store,
		vector:     cfg.Vector,
		summarizer: cfg.Summarizer,
		threshold:  threshold,
	}
}

// ConsolidateStats holds the results of a consolidation run.
type ConsolidateStats struct {
	Checked int
	Merged  int
	Errors  int
}

// Run finds and merges similar memory clusters.
func (c *Consolidator) Run(ctx context.Context) (*ConsolidateStats, error) {
	entries, err := c.store.List()
	if err != nil {
		return nil, fmt.Errorf("consolidate: list: %w", err)
	}

	stats := &ConsolidateStats{Checked: len(entries)}
	merged := make(map[string]bool) // track already-merged IDs

	for _, entry := range entries {
		if ctx.Err() != nil {
			break
		}
		if merged[entry.ID] {
			continue
		}

		// Find similar memories via vector search
		if c.vector == nil {
			break // no vector store → can't find similar
		}

		_, content, err := c.store.Get(entry.ID)
		if err != nil {
			continue
		}

		text := BuildEmbedText(entry, content)
		results, err := c.vector.Query(ctx, text, 5)
		if err != nil {
			continue
		}

		// Collect candidates above threshold (excluding self)
		var candidates []string
		for _, r := range results {
			if r.ID == entry.ID || merged[r.ID] {
				continue
			}
			if float64(r.Similarity) >= c.threshold {
				candidates = append(candidates, r.ID)
			}
		}

		if len(candidates) == 0 {
			continue
		}

		// Build merge group: entry + candidates
		group := []string{entry.ID}
		group = append(group, candidates...)

		if err := c.mergeGroup(ctx, group); err != nil {
			slog.Warn("consolidate: merge failed", "group", group, "error", err)
			stats.Errors++
			continue
		}

		// Mark all as merged
		for _, id := range group {
			merged[id] = true
		}
		stats.Merged++
	}

	return stats, nil
}

func (c *Consolidator) mergeGroup(ctx context.Context, ids []string) error {
	// Load all entries
	var memoryTexts []string
	for _, id := range ids {
		entry, content, err := c.store.Get(id)
		if err != nil {
			continue
		}
		memoryTexts = append(memoryTexts, fmt.Sprintf(
			"[%s] %s (type: %s, tags: %s)\n%s",
			entry.ID, entry.Title, entry.Type,
			strings.Join(entry.Tags, ", "), content,
		))
	}

	if len(memoryTexts) < 2 {
		return nil // need at least 2 to merge
	}

	// Call LLM to merge
	prompt := fmt.Sprintf(consolidationPrompt, strings.Join(memoryTexts, "\n---\n"))
	response, err := c.summarizer.Summarize(ctx, prompt)
	if err != nil {
		return fmt.Errorf("llm merge: %w", err)
	}

	// Parse response
	var merged struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
		Type    string   `json:"type"`
	}

	// Handle markdown fences
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		if len(lines) > 2 {
			response = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	if err := json.Unmarshal([]byte(response), &merged); err != nil {
		return fmt.Errorf("parse merge result: %w", err)
	}

	// Create consolidated entry
	newEntry := &MemoryEntry{
		Title:      merged.Title,
		Type:       MemoryType(merged.Type),
		Source:     "consolidation",
		Tags:       merged.Tags,
		Importance: ImportanceNormal,
		Confidence: 0.8,
	}
	if err := c.store.Create(newEntry, merged.Content); err != nil {
		return fmt.Errorf("create merged: %w", err)
	}

	// Mark source entries as merged
	now := time.Now()
	for _, id := range ids {
		entry, content, err := c.store.Get(id)
		if err != nil {
			continue
		}
		entry.MergedInto = newEntry.ID
		entry.UpdatedAt = now
		_ = c.store.Update(entry, content)
	}

	slog.Info("consolidated memories",
		"sources", ids,
		"target", newEntry.ID,
		"title", newEntry.Title,
	)

	return nil
}
