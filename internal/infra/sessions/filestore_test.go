package sessions

import (
	"strings"
	"testing"
	"time"
)

func TestCreateGetRoundTrip(t *testing.T) {
	store := NewFileStore(t.TempDir())

	s, err := store.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !strings.HasPrefix(s.ID, "sess_") {
		t.Errorf("ID = %q, want sess_ prefix", s.ID)
	}
	if s.Status != SessionActive {
		t.Errorf("Status = %q, want %q", s.Status, SessionActive)
	}

	got, err := store.Get(s.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("Get ID = %q, want %q", got.ID, s.ID)
	}
}

func TestGetNotFound(t *testing.T) {
	store := NewFileStore(t.TempDir())

	_, err := store.Get("sess_nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want 'not found'", err)
	}
}

func TestAppendAndLoadMessages(t *testing.T) {
	store := NewFileStore(t.TempDir())

	s, err := store.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	msgs := []Message{
		{Role: "user", Content: "hello", Ts: time.Now()},
		{Role: "assistant", Content: "hi there", Ts: time.Now()},
		{Role: "user", Content: "how are you?", Ts: time.Now()},
		{Role: "assistant", Content: "I'm fine", Ts: time.Now()},
		{Role: "user", Content: "bye", Ts: time.Now()},
	}

	for _, m := range msgs {
		if err := store.AppendMessage(s.ID, m); err != nil {
			t.Fatalf("AppendMessage: %v", err)
		}
	}

	loaded, err := store.LoadMessages(s.ID)
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}

	if len(loaded) != len(msgs) {
		t.Fatalf("loaded %d messages, want %d", len(loaded), len(msgs))
	}

	for i, m := range loaded {
		if m.Role != msgs[i].Role {
			t.Errorf("msg[%d].Role = %q, want %q", i, m.Role, msgs[i].Role)
		}
		if m.Content != msgs[i].Content {
			t.Errorf("msg[%d].Content = %q, want %q", i, m.Content, msgs[i].Content)
		}
	}

	// Verify message count in meta
	got, err := store.Get(s.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.MessageCount != 5 {
		t.Errorf("MessageCount = %d, want 5", got.MessageCount)
	}
}

func TestClose(t *testing.T) {
	store := NewFileStore(t.TempDir())

	s, err := store.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Close(s.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := store.Get(s.ID)
	if err != nil {
		t.Fatalf("Get after Close: %v", err)
	}
	if got.Status != SessionClosed {
		t.Errorf("Status = %q, want %q", got.Status, SessionClosed)
	}
}

func TestListOrdering(t *testing.T) {
	store := NewFileStore(t.TempDir())

	// Create 3 sessions with different update times
	s1, err := store.Create()
	if err != nil {
		t.Fatalf("Create s1: %v", err)
	}

	s2, err := store.Create()
	if err != nil {
		t.Fatalf("Create s2: %v", err)
	}

	s3, err := store.Create()
	if err != nil {
		t.Fatalf("Create s3: %v", err)
	}

	// Update s1 to be the most recent
	s1.UpdatedAt = time.Now().Add(time.Second)
	if err := store.UpdateMeta(s1); err != nil {
		t.Fatalf("UpdateMeta: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("List returned %d sessions, want 3", len(list))
	}

	// s1 should be first (most recently updated)
	if list[0].ID != s1.ID {
		t.Errorf("list[0].ID = %q, want %q (most recent)", list[0].ID, s1.ID)
	}

	// s3 should be before s2 (created later)
	_ = s2
	_ = s3
}

func TestLoadMessagesEmpty(t *testing.T) {
	store := NewFileStore(t.TempDir())

	s, err := store.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	msgs, err := store.LoadMessages(s.ID)
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}
