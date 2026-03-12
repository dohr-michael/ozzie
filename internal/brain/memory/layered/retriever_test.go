package layered

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeTestIndex() *Index {
	now := time.Now()
	return &Index{
		Version:   1,
		SessionID: "sess_ret",
		Root: Root{
			ID:       "root",
			Abstract: "discussion about golang programming and web development",
			Summary:  "conversation covered golang concurrency patterns, web API design, and database optimization",
			Keywords: []string{"golang", "concurrency", "web", "api", "database"},
			ChildIDs: []string{"n1", "n2", "n3"},
		},
		Nodes: []Node{
			{
				ID:       "n1",
				Abstract: "golang concurrency with goroutines and channels",
				Summary:  "discussed goroutines, channels, select statements, and common concurrency patterns in Go. Covered mutex usage and race condition prevention.",
				Keywords: []string{"golang", "goroutines", "channels", "concurrency", "mutex"},
				Metadata: NodeMetadata{MessageCount: 8, RecencyRank: 2},
				TokenEstimate: TokenEstimate{
					Abstract:   15,
					Summary:    40,
					Transcript: 500,
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:       "n2",
				Abstract: "REST API design best practices",
				Summary:  "covered RESTful API design, HTTP methods, status codes, pagination, and error handling patterns for web services.",
				Keywords: []string{"rest", "api", "http", "design", "web"},
				Metadata: NodeMetadata{MessageCount: 8, RecencyRank: 1},
				TokenEstimate: TokenEstimate{
					Abstract:   12,
					Summary:    35,
					Transcript: 450,
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:       "n3",
				Abstract: "database query optimization techniques",
				Summary:  "discussed SQL query optimization, indexing strategies, query plans, and N+1 problem solutions for PostgreSQL.",
				Keywords: []string{"database", "sql", "optimization", "indexing", "postgresql"},
				Metadata: NodeMetadata{MessageCount: 8, RecencyRank: 0},
				TokenEstimate: TokenEstimate{
					Abstract:   12,
					Summary:    35,
					Transcript: 400,
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestRetrieverRetrieve(t *testing.T) {
	dir := t.TempDir()
	sessionID := "sess_ret"
	if err := os.MkdirAll(filepath.Join(dir, sessionID), 0o755); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dir)
	cfg := DefaultConfig()
	cfg.MaxPromptTokens = 10000
	retriever := NewRetriever(store, cfg)

	index := makeTestIndex()
	result, err := retriever.Retrieve(sessionID, index, "goroutines and channels in golang")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if len(result.Selections) == 0 {
		t.Fatal("expected at least one selection")
	}

	// The golang concurrency node should be selected
	found := false
	for _, sel := range result.Selections {
		if sel.NodeID == "n1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected node n1 (golang concurrency) to be selected")
	}
}

func TestRetrieverEmptyIndex(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	cfg := DefaultConfig()
	retriever := NewRetriever(store, cfg)

	result, err := retriever.Retrieve("sess_empty", nil, "anything")
	if err != nil {
		t.Fatalf("Retrieve nil index: %v", err)
	}
	if len(result.Selections) != 0 {
		t.Error("expected no selections for nil index")
	}
}

func TestRetrieverBudgetRespected(t *testing.T) {
	dir := t.TempDir()
	sessionID := "sess_budget"
	if err := os.MkdirAll(filepath.Join(dir, sessionID), 0o755); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dir)
	cfg := DefaultConfig()
	cfg.MaxPromptTokens = 1000 // Very small budget
	retriever := NewRetriever(store, cfg)

	index := makeTestIndex()
	result, err := retriever.Retrieve(sessionID, index, "golang")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	// Total tokens should not exceed budget
	budget := int(float64(cfg.MaxPromptTokens) * 0.45)
	if result.TokenUsage.Used > budget {
		t.Errorf("used %d tokens, exceeds budget %d", result.TokenUsage.Used, budget)
	}
}
