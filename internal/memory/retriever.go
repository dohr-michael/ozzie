package memory

import (
	"math"
	"sort"
	"strings"
	"time"
)

// RetrievedMemory wraps a memory entry with retrieval score.
type RetrievedMemory struct {
	Entry   *MemoryEntry
	Content string
	Score   float64
}

// Retriever performs keyword-based memory retrieval with scoring.
type Retriever struct {
	store Store
}

// NewRetriever creates a new Retriever.
func NewRetriever(store Store) *Retriever {
	return &Retriever{store: store}
}

// Retrieve finds the most relevant memories for the given query.
// Scoring: tag match × 3 + title word match × 2 + recency bonus × confidence.
func (r *Retriever) Retrieve(query string, tags []string, limit int) ([]RetrievedMemory, error) {
	entries, err := r.store.List()
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 5
	}

	queryWords := tokenize(query)
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}

	var results []RetrievedMemory
	for _, entry := range entries {
		score := r.scoreEntry(entry, queryWords, tagSet)
		if score <= 0 {
			continue
		}

		content, err := r.readContent(entry.ID)
		if err != nil {
			continue
		}

		results = append(results, RetrievedMemory{
			Entry:   entry,
			Content: content,
			Score:   score,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (r *Retriever) scoreEntry(entry *MemoryEntry, queryWords []string, filterTags map[string]bool) float64 {
	var score float64

	// Tag match × 3
	for _, tag := range entry.Tags {
		lower := strings.ToLower(tag)
		if filterTags[lower] {
			score += 3.0
		}
		for _, qw := range queryWords {
			if lower == qw {
				score += 3.0
			}
		}
	}

	// Title word match × 2
	titleWords := tokenize(entry.Title)
	for _, tw := range titleWords {
		for _, qw := range queryWords {
			if tw == qw {
				score += 2.0
			}
		}
	}

	// Recency bonus
	score += recencyBonus(entry.LastUsedAt)

	// Apply confidence multiplier
	score *= math.Max(entry.Confidence, 0.1)

	return score
}

func (r *Retriever) readContent(id string) (string, error) {
	_, content, err := r.store.Get(id)
	if err != nil {
		return "", err
	}
	return content, nil
}

// recencyBonus returns a bonus based on how recently the memory was used.
func recencyBonus(lastUsed time.Time) float64 {
	days := time.Since(lastUsed).Hours() / 24
	switch {
	case days < 7:
		return 1.0
	case days < 30:
		return 0.5
	default:
		return 0.1
	}
}

// tokenize splits a string into lowercase words.
func tokenize(s string) []string {
	words := strings.Fields(strings.ToLower(s))
	result := make([]string, 0, len(words))
	for _, w := range words {
		// Strip common punctuation
		w = strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(w) > 1 {
			result = append(result, w)
		}
	}
	return result
}
