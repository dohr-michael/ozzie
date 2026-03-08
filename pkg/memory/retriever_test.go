package memory

import (
	"fmt"
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/pkg/names"
)

// inMemoryStore is a minimal Store for retriever tests (no CGo needed).
type inMemoryStore struct {
	entries []*MemoryEntry
	content map[string]string
}

func newInMemoryStore() *inMemoryStore {
	return &inMemoryStore{content: make(map[string]string)}
}

func (s *inMemoryStore) Create(entry *MemoryEntry, content string) error {
	if entry.ID == "" {
		entry.ID = names.GenerateID("mem", func(string) bool { return false })
	}
	now := time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = now
	}
	if entry.LastUsedAt.IsZero() {
		entry.LastUsedAt = now
	}
	if entry.Confidence == 0 {
		entry.Confidence = 0.8
	}
	s.entries = append(s.entries, entry)
	s.content[entry.ID] = content
	return nil
}

func (s *inMemoryStore) Get(id string) (*MemoryEntry, string, error) {
	for _, e := range s.entries {
		if e.ID == id {
			return e, s.content[id], nil
		}
	}
	return nil, "", fmt.Errorf("memory %q not found", id)
}

func (s *inMemoryStore) Update(entry *MemoryEntry, content string) error {
	for i, e := range s.entries {
		if e.ID == entry.ID {
			s.entries[i] = entry
			s.content[entry.ID] = content
			return nil
		}
	}
	return fmt.Errorf("memory %q not found", entry.ID)
}

func (s *inMemoryStore) Delete(id string) error {
	for i, e := range s.entries {
		if e.ID == id {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			delete(s.content, id)
			return nil
		}
	}
	return fmt.Errorf("memory %q not found", id)
}

func (s *inMemoryStore) List() ([]*MemoryEntry, error) {
	out := make([]*MemoryEntry, len(s.entries))
	copy(out, s.entries)
	return out, nil
}

func seedMemories(t *testing.T, store Store) {
	t.Helper()

	now := time.Now()
	entries := []struct {
		entry   *MemoryEntry
		content string
	}{
		{
			entry: &MemoryEntry{
				Title:      "Prefers tabs",
				Type:       MemoryPreference,
				Source:     "user",
				Tags:       []string{"editor", "formatting"},
				Confidence: 0.9,
				LastUsedAt: now,
			},
			content: "The user prefers tabs over spaces for indentation.",
		},
		{
			entry: &MemoryEntry{
				Title:      "Go project structure",
				Type:       MemoryFact,
				Source:     "agent",
				Tags:       []string{"golang", "project"},
				Confidence: 0.7,
				LastUsedAt: now.Add(-10 * 24 * time.Hour), // 10 days ago
			},
			content: "The project uses internal/ package layout with cmd/ for entry points.",
		},
		{
			entry: &MemoryEntry{
				Title:      "Deploy procedure",
				Type:       MemoryProcedure,
				Source:     "user",
				Tags:       []string{"deploy", "ops"},
				Confidence: 0.85,
				LastUsedAt: now.Add(-60 * 24 * time.Hour), // 60 days ago
			},
			content: "Run `make build && make deploy` to deploy to production.",
		},
	}

	for _, e := range entries {
		if err := store.Create(e.entry, e.content); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func TestRetriever_BasicQuery(t *testing.T) {
	store := newInMemoryStore()
	seedMemories(t, store)

	r := NewRetriever(store)
	results, err := r.Retrieve("tabs formatting", nil, 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	// "Prefers tabs" should be the top result (tag match + title match + recency)
	if results[0].Entry.Title != "Prefers tabs" {
		t.Fatalf("expected top result 'Prefers tabs', got %q", results[0].Entry.Title)
	}
}

func TestRetriever_TagFilter(t *testing.T) {
	store := newInMemoryStore()
	seedMemories(t, store)

	r := NewRetriever(store)
	results, err := r.Retrieve("something", []string{"deploy"}, 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one result with deploy tag")
	}

	foundDeploy := false
	for _, rm := range results {
		if rm.Entry.Title == "Deploy procedure" {
			foundDeploy = true
			break
		}
	}
	if !foundDeploy {
		t.Fatal("expected 'Deploy procedure' in results for deploy tag")
	}
}

func TestRetriever_LimitResults(t *testing.T) {
	store := newInMemoryStore()
	seedMemories(t, store)

	r := NewRetriever(store)
	results, err := r.Retrieve("project editor deploy", nil, 2)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if len(results) > 2 {
		t.Fatalf("expected at most 2 results, got %d", len(results))
	}
}

func TestRetriever_EmptyQuery(t *testing.T) {
	store := newInMemoryStore()
	seedMemories(t, store)

	r := NewRetriever(store)
	results, err := r.Retrieve("", nil, 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	// With empty query, recency bonus still applies → all entries score > 0
	if len(results) != 3 {
		t.Fatalf("expected 3 results for empty query (recency bonus), got %d", len(results))
	}
}

func TestRetriever_EmptyStore(t *testing.T) {
	store := newInMemoryStore()

	r := NewRetriever(store)
	results, err := r.Retrieve("test query", nil, 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty store, got %d", len(results))
	}
}

func TestTokenize(t *testing.T) {
	words := tokenize("Hello, World! This is a test.")
	expected := []string{"hello", "world", "this", "is", "test"}

	if len(words) != len(expected) {
		t.Fatalf("expected %d words, got %d: %v", len(expected), len(words), words)
	}
	for i, w := range expected {
		if words[i] != w {
			t.Fatalf("word %d: expected %q, got %q", i, w, words[i])
		}
	}
}
