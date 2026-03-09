// Package layered implements a three-tier progressive context compression
// system (L0 abstract, L1 summary, L2 full transcript) with BM25-based
// retrieval for relevance-aware context selection.
package layeredctx

import "time"

// Config configures the layered context system.
type Config struct {
	Enabled            bool
	L0TargetTokens     int
	L1TargetTokens     int
	MaxPromptTokens    int
	ScoreThresholdHigh float64
	Top1Top2Margin     float64
	MaxItemsL1         int
	MaxItemsL2         int
	MaxArchives        int
	MaxRecentMessages  int
	ArchiveChunkSize   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:            false,
		L0TargetTokens:     120,
		L1TargetTokens:     1200,
		MaxPromptTokens:    100000,
		ScoreThresholdHigh: 0.64,
		Top1Top2Margin:     0.08,
		MaxItemsL1:         4,
		MaxItemsL2:         2,
		MaxArchives:        12,
		MaxRecentMessages:  24,
		ArchiveChunkSize:   8,
	}
}

// NodeMetadata holds auxiliary info about a node.
type NodeMetadata struct {
	MessageCount int `json:"message_count"`
	RecencyRank  int `json:"recency_rank"`
}

// TokenEstimate stores token counts for each layer.
type TokenEstimate struct {
	Abstract   int `json:"abstract"`
	Summary    int `json:"summary"`
	Transcript int `json:"transcript"`
}

// Node represents one archived chunk of conversation.
type Node struct {
	ID            string        `json:"id"`
	Abstract      string        `json:"abstract"`
	Summary       string        `json:"summary"`
	ResourcePath  string        `json:"resource_path"`
	Checksum      string        `json:"checksum"`
	Keywords      []string      `json:"keywords"`
	Metadata      NodeMetadata  `json:"metadata"`
	TokenEstimate TokenEstimate `json:"token_estimate"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// Root is the summary of all nodes for a session.
type Root struct {
	ID       string   `json:"id"`
	Abstract string   `json:"abstract"`
	Summary  string   `json:"summary"`
	Keywords []string `json:"keywords"`
	ChildIDs []string `json:"child_ids"`
}

// Index is the persisted document per session.
type Index struct {
	Version   int       `json:"version"`
	SessionID string    `json:"session_id"`
	Root      Root      `json:"root"`
	Nodes     []Node    `json:"nodes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ArchivePayload stores the full transcript for a node.
type ArchivePayload struct {
	NodeID     string `json:"node_id"`
	Transcript string `json:"transcript"`
}

// Layer represents a retrieval depth.
type Layer string

const (
	LayerL0 Layer = "L0"
	LayerL1 Layer = "L1"
	LayerL2 Layer = "L2"
)

// Selection is a single item chosen for inclusion in the prompt.
type Selection struct {
	NodeID  string `json:"node_id"`
	Layer   Layer  `json:"layer"`
	Content string `json:"content"`
	Tokens  int    `json:"tokens"`
	Score   float64 `json:"score"`
}

// RetrievalDecision records why the retriever chose a particular depth.
type RetrievalDecision struct {
	ReachedLayer Layer   `json:"reached_layer"`
	TopScore     float64 `json:"top_score"`
	Reason       string  `json:"reason"`
}

// TokenUsage tracks token consumption for a retrieval pass.
type TokenUsage struct {
	Budget       int     `json:"budget"`
	Used         int     `json:"used"`
	Total        int     `json:"total"`
	SavingsRatio float64 `json:"savings_ratio"`
}

// RetrievalResult holds the output of a retrieval pass.
type RetrievalResult struct {
	Selections []Selection       `json:"selections"`
	Decision   RetrievalDecision `json:"decision"`
	TokenUsage TokenUsage        `json:"token_usage"`
}
