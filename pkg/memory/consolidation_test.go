package memory

import (
	"context"
	"encoding/json"
	"testing"
)

type consolidationMockSummarizer struct {
	response string
}

func (m *consolidationMockSummarizer) Summarize(_ context.Context, _ string) (string, error) {
	return m.response, nil
}

func TestConsolidator_MergeGroup(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	// Create similar entries
	e1 := &MemoryEntry{Title: "Go concurrency", Type: MemoryFact, Source: "test", Tags: []string{"go"}}
	_ = store.Create(e1, "Goroutines and channels for concurrent programming")

	e2 := &MemoryEntry{Title: "Go goroutines", Type: MemoryFact, Source: "test", Tags: []string{"go"}}
	_ = store.Create(e2, "Using goroutines for concurrent tasks")

	// Mock LLM response
	mergedResp, _ := json.Marshal(map[string]any{
		"title":   "Go Concurrency Patterns",
		"content": "Go provides goroutines and channels for concurrent programming.",
		"tags":    []string{"go", "concurrency"},
		"type":    "fact",
	})

	c := NewConsolidator(ConsolidatorConfig{
		Store:      store,
		Summarizer: &consolidationMockSummarizer{response: string(mergedResp)},
	})

	err = c.mergeGroup(context.Background(), []string{e1.ID, e2.ID})
	if err != nil {
		t.Fatalf("mergeGroup: %v", err)
	}

	// Verify sources are marked as merged
	s1, _, _ := store.Get(e1.ID)
	if s1.MergedInto == "" {
		t.Error("expected e1 to be merged")
	}
	s2, _, _ := store.Get(e2.ID)
	if s2.MergedInto == "" {
		t.Error("expected e2 to be merged")
	}

	// Verify merged entry exists and sources are excluded from List
	entries, _ := store.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 active entry (merged), got %d", len(entries))
	}
	if entries[0].Title != "Go Concurrency Patterns" {
		t.Errorf("expected merged title, got %q", entries[0].Title)
	}
}
