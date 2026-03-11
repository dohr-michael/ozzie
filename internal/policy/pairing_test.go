package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dohr-michael/ozzie/pkg/connector"
)

func TestPairingAddAndResolve(t *testing.T) {
	dir := t.TempDir()
	s := NewPairingStore(dir)

	err := s.Add(Pairing{
		Key:        PairingKey{Platform: "discord", ServerID: "guild1", ChannelID: "ch1", UserID: "user1"},
		PolicyName: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}

	name, ok := s.Resolve(connector.Identity{
		Platform:  "discord",
		ServerID:  "guild1",
		ChannelID: "ch1",
		UserID:    "user1",
	})
	if !ok || name != "admin" {
		t.Errorf("got (%q, %v), want (admin, true)", name, ok)
	}
}

func TestPairingSpecificity(t *testing.T) {
	dir := t.TempDir()
	s := NewPairingStore(dir)

	// Platform-wide wildcard
	_ = s.Add(Pairing{
		Key:        PairingKey{Platform: "discord", ServerID: "*", ChannelID: "*", UserID: "*"},
		PolicyName: "readonly",
	})
	// Server-wide wildcard
	_ = s.Add(Pairing{
		Key:        PairingKey{Platform: "discord", ServerID: "guild1", ChannelID: "*", UserID: "*"},
		PolicyName: "support",
	})
	// Channel-wide wildcard
	_ = s.Add(Pairing{
		Key:        PairingKey{Platform: "discord", ServerID: "guild1", ChannelID: "ch1", UserID: "*"},
		PolicyName: "executor",
	})
	// Exact match
	_ = s.Add(Pairing{
		Key:        PairingKey{Platform: "discord", ServerID: "guild1", ChannelID: "ch1", UserID: "user1"},
		PolicyName: "admin",
	})

	tests := []struct {
		name string
		id   connector.Identity
		want string
	}{
		{
			name: "exact match",
			id:   connector.Identity{Platform: "discord", ServerID: "guild1", ChannelID: "ch1", UserID: "user1"},
			want: "admin",
		},
		{
			name: "channel wildcard",
			id:   connector.Identity{Platform: "discord", ServerID: "guild1", ChannelID: "ch1", UserID: "user999"},
			want: "executor",
		},
		{
			name: "server wildcard",
			id:   connector.Identity{Platform: "discord", ServerID: "guild1", ChannelID: "other", UserID: "user999"},
			want: "support",
		},
		{
			name: "platform wildcard",
			id:   connector.Identity{Platform: "discord", ServerID: "guild2", ChannelID: "ch1", UserID: "user1"},
			want: "readonly",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := s.Resolve(tt.id)
			if !ok {
				t.Fatal("expected match")
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPairingNotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewPairingStore(dir)

	_, ok := s.Resolve(connector.Identity{Platform: "slack", UserID: "u1"})
	if ok {
		t.Error("expected no match for empty store")
	}
}

func TestPairingRemove(t *testing.T) {
	dir := t.TempDir()
	s := NewPairingStore(dir)

	key := PairingKey{Platform: "discord", ServerID: "*", ChannelID: "*", UserID: "*"}
	_ = s.Add(Pairing{Key: key, PolicyName: "readonly"})

	if err := s.Remove(key); err != nil {
		t.Fatal(err)
	}

	list := s.List()
	if len(list) != 0 {
		t.Errorf("expected empty list after remove, got %d", len(list))
	}
}

func TestPairingUpdateExisting(t *testing.T) {
	dir := t.TempDir()
	s := NewPairingStore(dir)

	key := PairingKey{Platform: "discord", ServerID: "*", ChannelID: "*", UserID: "*"}
	_ = s.Add(Pairing{Key: key, PolicyName: "readonly"})
	_ = s.Add(Pairing{Key: key, PolicyName: "admin"})

	list := s.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 pairing after update, got %d", len(list))
	}
	if list[0].PolicyName != "admin" {
		t.Errorf("policy = %q, want admin", list[0].PolicyName)
	}
}

func TestPairingPersistence(t *testing.T) {
	dir := t.TempDir()
	s := NewPairingStore(dir)

	_ = s.Add(Pairing{
		Key:        PairingKey{Platform: "web", ServerID: "*", ChannelID: "*", UserID: "alice"},
		PolicyName: "admin",
	})

	// Reload from disk
	s2 := NewPairingStore(dir)
	list := s2.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 pairing after reload, got %d", len(list))
	}
	if list[0].PolicyName != "admin" {
		t.Errorf("policy = %q after reload, want admin", list[0].PolicyName)
	}
}

func TestPairingList(t *testing.T) {
	dir := t.TempDir()
	s := NewPairingStore(dir)

	_ = s.Add(Pairing{Key: PairingKey{Platform: "a", ServerID: "*", ChannelID: "*", UserID: "*"}, PolicyName: "p1"})
	_ = s.Add(Pairing{Key: PairingKey{Platform: "b", ServerID: "*", ChannelID: "*", UserID: "*"}, PolicyName: "p2"})

	list := s.List()
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
}

func TestPairingStoreCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	s := NewPairingStore(dir)

	_ = s.Add(Pairing{
		Key:        PairingKey{Platform: "x", ServerID: "*", ChannelID: "*", UserID: "*"},
		PolicyName: "admin",
	})

	if _, err := os.Stat(filepath.Join(dir, "pairings.json")); err != nil {
		t.Errorf("expected pairings.json to exist: %v", err)
	}
}
