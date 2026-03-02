package layered

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/sessions"
)

func TestManagerApplyShortHistory(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	cfg := DefaultConfig()
	cfg.MaxRecentMessages = 24
	mgr := NewManager(store, cfg, nil)

	msgs := []*schema.Message{
		{Role: schema.User, Content: "hello"},
		{Role: schema.Assistant, Content: "hi there"},
	}
	history := []sessions.Message{
		{Role: "user", Content: "hello", Ts: time.Now()},
		{Role: "assistant", Content: "hi there", Ts: time.Now()},
	}

	result, err := mgr.Apply(context.Background(), "sess_short", msgs, history)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Short history — should return messages unchanged
	if len(result) != len(msgs) {
		t.Errorf("len(result) = %d, want %d", len(result), len(msgs))
	}
}

func TestManagerApplyLongHistory(t *testing.T) {
	dir := t.TempDir()
	sessionID := "sess_long"
	if err := os.MkdirAll(filepath.Join(dir, sessionID), 0o755); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dir)
	cfg := DefaultConfig()
	cfg.MaxRecentMessages = 4
	cfg.ArchiveChunkSize = 4
	cfg.MaxPromptTokens = 50000
	mgr := NewManager(store, cfg, nil)

	// Build a long history
	history := make([]sessions.Message, 20)
	for i := range history {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		history[i] = sessions.Message{
			Role:    role,
			Content: "message about golang programming patterns and best practices number " + string(rune('A'+i%26)),
			Ts:      time.Now(),
		}
	}

	// Current messages (the schema versions)
	msgs := make([]*schema.Message, len(history))
	for i, h := range history {
		msgs[i] = h.ToSchemaMessage()
	}

	result, err := mgr.Apply(context.Background(), sessionID, msgs, history)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Should have fewer messages than original (archived + recent)
	if len(result) >= len(msgs) {
		t.Errorf("expected compression: result=%d, original=%d", len(result), len(msgs))
	}

	// First message should be the layered context
	if len(result) > 0 && result[0].Role != schema.User {
		t.Errorf("first message role = %q, want %q", result[0].Role, schema.User)
	}

	// Should contain layered context marker
	if len(result) > 0 {
		found := false
		for _, m := range result {
			if m.Content != "" && len(m.Content) > 10 {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected at least one non-trivial message in result")
		}
	}
}

func TestManagerApplyCreatesIndex(t *testing.T) {
	dir := t.TempDir()
	sessionID := "sess_creates_idx"
	if err := os.MkdirAll(filepath.Join(dir, sessionID), 0o755); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dir)
	cfg := DefaultConfig()
	cfg.MaxRecentMessages = 4
	cfg.ArchiveChunkSize = 4
	mgr := NewManager(store, cfg, nil)

	history := make([]sessions.Message, 12)
	for i := range history {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		history[i] = sessions.Message{Role: role, Content: "test content", Ts: time.Now()}
	}
	msgs := make([]*schema.Message, len(history))
	for i, h := range history {
		msgs[i] = h.ToSchemaMessage()
	}

	_, err := mgr.Apply(context.Background(), sessionID, msgs, history)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Index should now exist on disk
	idx, err := store.LoadIndex(sessionID)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if idx == nil {
		t.Fatal("expected index to be created")
	}
	if len(idx.Nodes) == 0 {
		t.Error("expected nodes in index")
	}
}
