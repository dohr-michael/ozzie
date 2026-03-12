package eyes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionMapGetSet(t *testing.T) {
	dir := t.TempDir()
	m := NewSessionMap(dir)

	// Not found
	if _, ok := m.Get("discord", "g1", "c1", "u1"); ok {
		t.Fatal("expected not found")
	}

	// Set and get
	if err := m.Set("discord", "g1", "c1", "u1", "sess_a"); err != nil {
		t.Fatal(err)
	}
	sid, ok := m.Get("discord", "g1", "c1", "u1")
	if !ok || sid != "sess_a" {
		t.Fatalf("got %q, want sess_a", sid)
	}

	// Overwrite
	if err := m.Set("discord", "g1", "c1", "u1", "sess_b"); err != nil {
		t.Fatal(err)
	}
	sid, ok = m.Get("discord", "g1", "c1", "u1")
	if !ok || sid != "sess_b" {
		t.Fatalf("got %q, want sess_b", sid)
	}
}

func TestSessionMapPersistence(t *testing.T) {
	dir := t.TempDir()
	m := NewSessionMap(dir)
	if err := m.Set("discord", "g1", "c1", "u1", "sess_a"); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	path := filepath.Join(dir, "connector_sessions.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file: %v", err)
	}

	// Reload
	m2 := NewSessionMap(dir)
	sid, ok := m2.Get("discord", "g1", "c1", "u1")
	if !ok || sid != "sess_a" {
		t.Fatalf("after reload: got %q, want sess_a", sid)
	}
}

func TestSessionMapBySession(t *testing.T) {
	dir := t.TempDir()
	m := NewSessionMap(dir)
	if err := m.Set("discord", "g1", "c1", "u1", "sess_a"); err != nil {
		t.Fatal(err)
	}

	conn, ch, ok := m.BySession("sess_a")
	if !ok {
		t.Fatal("expected found")
	}
	if conn != "discord" || ch != "c1" {
		t.Fatalf("got connector=%q channel=%q, want discord/c1", conn, ch)
	}

	// Not found
	_, _, ok = m.BySession("sess_z")
	if ok {
		t.Fatal("expected not found")
	}
}
