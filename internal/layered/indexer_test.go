package layered

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/sessions"
)

func makeMessages(n int) []sessions.Message {
	msgs := make([]sessions.Message, n)
	for i := range msgs {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs[i] = sessions.Message{
			Role:    role,
			Content: "message content number " + string(rune('A'+i%26)),
			Ts:      time.Now(),
		}
	}
	return msgs
}

func TestIndexerBuildOrUpdate(t *testing.T) {
	dir := t.TempDir()
	sessionID := "sess_idx1"
	if err := os.MkdirAll(filepath.Join(dir, sessionID), 0o755); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dir)
	cfg := DefaultConfig()
	cfg.ArchiveChunkSize = 4
	indexer := NewIndexer(store, NewLLMSummarizer(nil), cfg)

	msgs := makeMessages(16)
	index, err := indexer.BuildOrUpdate(context.Background(), sessionID, msgs)
	if err != nil {
		t.Fatalf("BuildOrUpdate: %v", err)
	}

	if index.SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", index.SessionID, sessionID)
	}
	// 16 messages / 4 chunk size = 4 nodes
	if len(index.Nodes) != 4 {
		t.Errorf("Nodes = %d, want 4", len(index.Nodes))
	}
	if index.Root.ID != "root" {
		t.Errorf("Root.ID = %q, want %q", index.Root.ID, "root")
	}
}

func TestIndexerCacheHit(t *testing.T) {
	dir := t.TempDir()
	sessionID := "sess_idx2"
	if err := os.MkdirAll(filepath.Join(dir, sessionID), 0o755); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dir)
	cfg := DefaultConfig()
	cfg.ArchiveChunkSize = 4
	indexer := NewIndexer(store, NewLLMSummarizer(nil), cfg)

	msgs := makeMessages(8)

	// First build
	idx1, err := indexer.BuildOrUpdate(context.Background(), sessionID, msgs)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}

	// Second build with same messages — should get cache hits
	idx2, err := indexer.BuildOrUpdate(context.Background(), sessionID, msgs)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}

	if len(idx1.Nodes) != len(idx2.Nodes) {
		t.Errorf("node count changed: %d → %d", len(idx1.Nodes), len(idx2.Nodes))
	}
	for i := range idx1.Nodes {
		if idx1.Nodes[i].Checksum != idx2.Nodes[i].Checksum {
			t.Errorf("checksum mismatch at %d", i)
		}
	}
}

func TestIndexerIncrementalRebuild(t *testing.T) {
	dir := t.TempDir()
	sessionID := "sess_idx3"
	if err := os.MkdirAll(filepath.Join(dir, sessionID), 0o755); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dir)
	cfg := DefaultConfig()
	cfg.ArchiveChunkSize = 4
	indexer := NewIndexer(store, NewLLMSummarizer(nil), cfg)

	msgs := makeMessages(8)
	idx1, err := indexer.BuildOrUpdate(context.Background(), sessionID, msgs)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}

	// Add more messages
	moreMsgs := append(msgs, makeMessages(4)...)
	idx2, err := indexer.BuildOrUpdate(context.Background(), sessionID, moreMsgs)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}

	if len(idx2.Nodes) <= len(idx1.Nodes) {
		t.Errorf("expected more nodes after adding messages: %d → %d", len(idx1.Nodes), len(idx2.Nodes))
	}
}

func TestChunkMessages(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ArchiveChunkSize = 3
	indexer := NewIndexer(nil, nil, cfg)

	msgs := makeMessages(7)
	chunks := indexer.chunkMessages(msgs)

	// 7 / 3 = 2 full chunks + 1 partial
	if len(chunks) != 3 {
		t.Errorf("chunks = %d, want 3", len(chunks))
	}
	if len(chunks[0]) != 3 {
		t.Errorf("chunk[0] size = %d, want 3", len(chunks[0]))
	}
	if len(chunks[2]) != 1 {
		t.Errorf("chunk[2] size = %d, want 1", len(chunks[2]))
	}
}

func TestLLMSummarizer(t *testing.T) {
	called := false
	mockLLM := func(_ context.Context, prompt string) (string, error) {
		called = true
		return "LLM summary of the conversation.", nil
	}

	summarizer := NewLLMSummarizer(mockLLM)
	result, err := summarizer(context.Background(), "some conversation text", 150)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected LLM to be called")
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestLLMSummarizerFallback(t *testing.T) {
	mockLLM := func(_ context.Context, prompt string) (string, error) {
		return "", errors.New("LLM unavailable")
	}

	summarizer := NewLLMSummarizer(mockLLM)
	result, err := summarizer(context.Background(), "First sentence. Second sentence. Third sentence.", 150)
	if err != nil {
		t.Fatalf("unexpected error (should fallback): %v", err)
	}
	if result == "" {
		t.Error("expected non-empty fallback result")
	}
	// Fallback should produce something from the heuristic
	expected := FallbackSummarizer("First sentence. Second sentence. Third sentence.", 150)
	if result != expected {
		t.Errorf("fallback result = %q, want %q", result, expected)
	}
}

func TestLLMSummarizerNil(t *testing.T) {
	summarizer := NewLLMSummarizer(nil)
	result, err := summarizer(context.Background(), "Some text here.", 150)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := FallbackSummarizer("Some text here.", 150)
	if result != expected {
		t.Errorf("nil LLM result = %q, want %q", result, expected)
	}
}
