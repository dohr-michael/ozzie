package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dohr-michael/ozzie/pkg/connector"
)

// PairingKey identifies a user×context combination.
// Wildcards: "*" matches any value for that field.
type PairingKey struct {
	Platform  string `json:"platform"`
	ServerID  string `json:"server_id"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
}

// Pairing maps a user×context to a policy name.
type Pairing struct {
	Key        PairingKey `json:"key"`
	PolicyName string     `json:"policy_name"`
}

// PairingStore manages identity→policy associations.
// Thread-safe; persisted as a JSON file.
type PairingStore struct {
	mu       sync.RWMutex
	pairings []Pairing
	path     string // JSON file path
}

// NewPairingStore creates a store that persists to dir/pairings.json.
func NewPairingStore(dir string) *PairingStore {
	s := &PairingStore{path: filepath.Join(dir, "pairings.json")}
	_ = s.load() // best-effort
	return s
}

// Add registers or updates a pairing and persists to disk.
func (s *PairingStore) Add(p Pairing) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Replace if same key exists
	for i, existing := range s.pairings {
		if existing.Key == p.Key {
			s.pairings[i] = p
			return s.save()
		}
	}
	s.pairings = append(s.pairings, p)
	return s.save()
}

// Remove deletes the pairing with the given key (if any) and persists.
func (s *PairingStore) Remove(key PairingKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, p := range s.pairings {
		if p.Key == key {
			s.pairings = append(s.pairings[:i], s.pairings[i+1:]...)
			return s.save()
		}
	}
	return nil // not found, no-op
}

// Resolve finds the most specific pairing matching the given identity.
// Returns the policy name and true, or ("", false) if no match.
//
// Specificity order (most to least):
//  1. Exact: platform + server + channel + user
//  2. Channel wildcard: platform + server + channel + *
//  3. Server wildcard: platform + server + * + *
//  4. Platform wildcard: platform + * + * + *
func (s *PairingStore) Resolve(id connector.Identity) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := []PairingKey{
		{Platform: id.Platform, ServerID: id.ServerID, ChannelID: id.ChannelID, UserID: id.UserID},
		{Platform: id.Platform, ServerID: id.ServerID, ChannelID: id.ChannelID, UserID: "*"},
		{Platform: id.Platform, ServerID: id.ServerID, ChannelID: "*", UserID: "*"},
		{Platform: id.Platform, ServerID: "*", ChannelID: "*", UserID: "*"},
	}

	for _, candidate := range candidates {
		for _, p := range s.pairings {
			if p.Key == candidate {
				return p.PolicyName, true
			}
		}
	}
	return "", false
}

// List returns all pairings.
func (s *PairingStore) List() []Pairing {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Pairing, len(s.pairings))
	copy(out, s.pairings)
	return out
}

func (s *PairingStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err // file doesn't exist yet — fine
	}
	return json.Unmarshal(data, &s.pairings)
}

func (s *PairingStore) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create pairing dir: %w", err)
	}
	data, err := json.MarshalIndent(s.pairings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pairings: %w", err)
	}
	return os.WriteFile(s.path, data, 0o644)
}
