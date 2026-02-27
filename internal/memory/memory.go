// Package memory provides long-term memory storage for Ozzie.
package memory

import (
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	decayGracePeriod = 7 * 24 * time.Hour // no decay for 7 days after last use
	decayRate        = 0.01               // per week after grace period
	decayFloor       = 0.1                // never below 0.1
)

// ApplyDecay reduces confidence based on time since last use.
// Grace period of 7 days, then -0.01 per week. Floor at 0.1.
func ApplyDecay(confidence float64, lastUsedAt time.Time, now time.Time) float64 {
	idle := now.Sub(lastUsedAt)
	if idle <= decayGracePeriod {
		return confidence
	}
	weeksIdle := (idle - decayGracePeriod).Hours() / (7 * 24)
	decayed := confidence - decayRate*weeksIdle
	return math.Max(decayed, decayFloor)
}

// MemoryType categorizes a memory entry.
type MemoryType string

const (
	MemoryPreference MemoryType = "preference"
	MemoryFact       MemoryType = "fact"
	MemoryProcedure  MemoryType = "procedure"
	MemoryContext    MemoryType = "context"
)

// MemoryEntry holds metadata for a single memory.
type MemoryEntry struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Source     string     `json:"source"`
	Type       MemoryType `json:"type"`
	Tags       []string   `json:"tags,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt time.Time  `json:"last_used_at"`
	Confidence float64    `json:"confidence"`

	// Embedding tracking — set after successful vector indexing.
	// Empty EmbeddingModel means the entry has never been indexed.
	EmbeddingModel string     `json:"embedding_model,omitempty"`
	IndexedAt      *time.Time `json:"indexed_at,omitempty"`
}

// IsIndexed returns true if this entry has been indexed with embeddings.
func (e *MemoryEntry) IsIndexed() bool {
	return e.EmbeddingModel != "" && e.IndexedAt != nil
}

// IsStale returns true if the entry content was updated after last indexing,
// or if the embedding model has changed.
func (e *MemoryEntry) IsStale(currentModel string) bool {
	if !e.IsIndexed() {
		return true
	}
	if e.EmbeddingModel != currentModel {
		return true
	}
	return e.IndexedAt.Before(e.UpdatedAt)
}

// generateMemoryID creates a unique memory identifier.
func generateMemoryID() string {
	u := uuid.New().String()
	return "mem_" + strings.ReplaceAll(u[:8], "-", "")
}
