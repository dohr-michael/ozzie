package scheduler

import (
	"fmt"
	"sort"
	"time"

	"github.com/dohr-michael/ozzie/internal/storage/dirstore"
)

// ScheduleStore persists schedule entries as directories with meta.json.
type ScheduleStore struct {
	ds *dirstore.DirStore
}

// NewScheduleStore creates a ScheduleStore rooted at baseDir.
func NewScheduleStore(baseDir string) *ScheduleStore {
	return &ScheduleStore{ds: dirstore.NewDirStore(baseDir, "schedule")}
}

// Create persists a new schedule entry to disk.
func (s *ScheduleStore) Create(entry *ScheduleEntry) error {
	s.ds.Lock()
	defer s.ds.Unlock()

	if entry.ID == "" {
		entry.ID = GenerateScheduleID()
	}

	entry.CreatedAt = time.Now()

	if err := s.ds.EnsureDir(entry.ID); err != nil {
		return err
	}

	return s.ds.WriteMeta(entry.ID, entry)
}

// Get reads a schedule entry by ID.
func (s *ScheduleStore) Get(id string) (*ScheduleEntry, error) {
	s.ds.RLock()
	defer s.ds.RUnlock()

	var entry ScheduleEntry
	if err := s.ds.ReadMeta(id, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// Update atomically rewrites a schedule entry's meta.json.
func (s *ScheduleStore) Update(entry *ScheduleEntry) error {
	s.ds.Lock()
	defer s.ds.Unlock()

	return s.ds.WriteMeta(entry.ID, entry)
}

// Delete removes a schedule entry directory.
func (s *ScheduleStore) Delete(id string) error {
	s.ds.Lock()
	defer s.ds.Unlock()

	return s.ds.RemoveDir(id)
}

// List returns all schedule entries, sorted by CreatedAt descending.
func (s *ScheduleStore) List() ([]*ScheduleEntry, error) {
	s.ds.RLock()
	defer s.ds.RUnlock()

	dirs, err := s.ds.ListDirs()
	if err != nil {
		return nil, err
	}

	var entries []*ScheduleEntry
	for _, name := range dirs {
		var entry ScheduleEntry
		if err := s.ds.ReadMeta(name, &entry); err != nil {
			continue // skip corrupted entries
		}
		entries = append(entries, &entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})

	return entries, nil
}

// ListBySession returns schedule entries for a specific session.
func (s *ScheduleStore) ListBySession(sessionID string) ([]*ScheduleEntry, error) {
	all, err := s.List()
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}

	var filtered []*ScheduleEntry
	for _, e := range all {
		if e.SessionID == sessionID {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}
