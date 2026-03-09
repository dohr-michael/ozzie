package layeredctx

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()

	s := NewStore(dir)
	sessionID := "sess_test1"

	// Create session directory
	if err := os.MkdirAll(filepath.Join(dir, sessionID), 0o755); err != nil {
		t.Fatal(err)
	}

	// Initially no index
	idx, err := s.LoadIndex(sessionID)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	if idx != nil {
		t.Fatal("expected nil index for new session")
	}

	// Save an index
	now := time.Now()
	index := &Index{
		Version:   1,
		SessionID: sessionID,
		Root: Root{
			ID:       "root",
			Abstract: "test abstract",
			Summary:  "test summary",
			Keywords: []string{"test", "layered"},
			ChildIDs: []string{"node_1"},
		},
		Nodes: []Node{
			{
				ID:       "node_1",
				Abstract: "node abstract",
				Summary:  "node summary",
				Checksum: "abc123",
				Keywords: []string{"golang"},
				Metadata: NodeMetadata{MessageCount: 8, RecencyRank: 0},
				TokenEstimate: TokenEstimate{
					Abstract:   30,
					Summary:    300,
					Transcript: 2000,
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.SaveIndex(sessionID, index); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}

	// Load it back
	loaded, err := s.LoadIndex(sessionID)
	if err != nil {
		t.Fatalf("LoadIndex after save: %v", err)
	}
	if loaded.SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", loaded.SessionID, sessionID)
	}
	if len(loaded.Nodes) != 1 {
		t.Fatalf("Nodes = %d, want 1", len(loaded.Nodes))
	}
	if loaded.Nodes[0].Checksum != "abc123" {
		t.Errorf("Checksum = %q, want %q", loaded.Nodes[0].Checksum, "abc123")
	}
}

func TestStoreArchiveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	sessionID := "sess_test2"

	if err := os.MkdirAll(filepath.Join(dir, sessionID), 0o755); err != nil {
		t.Fatal(err)
	}

	payload := ArchivePayload{
		NodeID:     "node_1",
		Transcript: "user: hello\nassistant: hi there",
	}

	if err := s.WriteArchive(sessionID, "node_1", payload); err != nil {
		t.Fatalf("WriteArchive: %v", err)
	}

	loaded, err := s.ReadArchive(sessionID, "node_1")
	if err != nil {
		t.Fatalf("ReadArchive: %v", err)
	}
	if loaded.Transcript != payload.Transcript {
		t.Errorf("Transcript mismatch: got %q", loaded.Transcript)
	}

	// Missing archive returns nil
	missing, err := s.ReadArchive(sessionID, "nonexistent")
	if err != nil {
		t.Fatalf("ReadArchive missing: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for missing archive")
	}
}

func TestStoreCleanupArchives(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	sessionID := "sess_test3"

	if err := os.MkdirAll(filepath.Join(dir, sessionID), 0o755); err != nil {
		t.Fatal(err)
	}

	// Write two archives
	s.WriteArchive(sessionID, "keep", ArchivePayload{NodeID: "keep", Transcript: "kept"})
	s.WriteArchive(sessionID, "remove", ArchivePayload{NodeID: "remove", Transcript: "removed"})

	// Cleanup: only "keep" is valid
	if err := s.CleanupArchives(sessionID, []string{"keep"}); err != nil {
		t.Fatalf("CleanupArchives: %v", err)
	}

	// "keep" should still exist
	kept, err := s.ReadArchive(sessionID, "keep")
	if err != nil || kept == nil {
		t.Error("kept archive should still exist")
	}

	// "remove" should be gone
	removed, err := s.ReadArchive(sessionID, "remove")
	if err != nil {
		t.Fatalf("ReadArchive removed: %v", err)
	}
	if removed != nil {
		t.Error("removed archive should be gone")
	}
}
