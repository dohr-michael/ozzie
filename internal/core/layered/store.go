package layered

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store persists layered context data under each session's directory.
// Layout:
//
//	~/.ozzie/sessions/sess_xxx/layered/index.json
//	~/.ozzie/sessions/sess_xxx/layered/archives/archive_<id>.json
type Store struct {
	sessionsDir string // base directory containing session subdirectories
}

// NewStore creates a Store rooted at the given sessions directory.
func NewStore(sessionsDir string) *Store {
	return &Store{sessionsDir: sessionsDir}
}

func (s *Store) layeredDir(sessionID string) string {
	return filepath.Join(s.sessionsDir, sessionID, "layered")
}

func (s *Store) archivesDir(sessionID string) string {
	return filepath.Join(s.sessionsDir, sessionID, "layered", "archives")
}

func (s *Store) indexPath(sessionID string) string {
	return filepath.Join(s.layeredDir(sessionID), "index.json")
}

func (s *Store) archivePath(sessionID, nodeID string) string {
	return filepath.Join(s.archivesDir(sessionID), fmt.Sprintf("archive_%s.json", nodeID))
}

// ensureDirs creates the layered/ and layered/archives/ directories if needed.
func (s *Store) ensureDirs(sessionID string) error {
	if err := os.MkdirAll(s.archivesDir(sessionID), 0o755); err != nil {
		return fmt.Errorf("create layered dirs: %w", err)
	}
	return nil
}

// LoadIndex reads the index.json for a session. Returns nil, nil if it doesn't exist.
func (s *Store) LoadIndex(sessionID string) (*Index, error) {
	data, err := os.ReadFile(s.indexPath(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read index: %w", err)
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("unmarshal index: %w", err)
	}
	return &idx, nil
}

// SaveIndex atomically writes the index.json for a session.
func (s *Store) SaveIndex(sessionID string, idx *Index) error {
	if err := s.ensureDirs(sessionID); err != nil {
		return err
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	path := s.indexPath(sessionID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write index tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename index: %w", err)
	}
	return nil
}

// WriteArchive writes a full transcript archive for a node.
func (s *Store) WriteArchive(sessionID, nodeID string, payload ArchivePayload) error {
	if err := s.ensureDirs(sessionID); err != nil {
		return err
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal archive: %w", err)
	}

	path := s.archivePath(sessionID, nodeID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write archive tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename archive: %w", err)
	}
	return nil
}

// ReadArchive reads the full transcript archive for a node.
func (s *Store) ReadArchive(sessionID, nodeID string) (*ArchivePayload, error) {
	data, err := os.ReadFile(s.archivePath(sessionID, nodeID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read archive: %w", err)
	}
	var payload ArchivePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal archive: %w", err)
	}
	return &payload, nil
}

// CleanupArchives removes archive files that are not in validNodeIDs.
func (s *Store) CleanupArchives(sessionID string, validNodeIDs []string) error {
	dir := s.archivesDir(sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read archives dir: %w", err)
	}

	valid := make(map[string]bool, len(validNodeIDs))
	for _, id := range validNodeIDs {
		valid[fmt.Sprintf("archive_%s.json", id)] = true
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !valid[entry.Name()] {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
	return nil
}
