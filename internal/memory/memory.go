// Package memory provides long-term memory storage for Ozzie.
package memory

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

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
}

// generateMemoryID creates a unique memory identifier.
func generateMemoryID() string {
	u := uuid.New().String()
	return "mem_" + strings.ReplaceAll(u[:8], "-", "")
}
