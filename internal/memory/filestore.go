package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileStore implements Store using filesystem storage.
// Structure:
//
//	<dir>/
//	  index.json       — []MemoryEntry metadata
//	  entries/
//	    mem_xxx.md     — content markdown
type FileStore struct {
	dir string

	mu    sync.RWMutex
	index []*MemoryEntry
}

// NewFileStore creates a FileStore and loads the index from disk.
func NewFileStore(dir string) *FileStore {
	fs := &FileStore{dir: dir}
	_ = fs.loadIndex()
	return fs
}

// Create adds a new memory entry with its content.
func (fs *FileStore) Create(entry *MemoryEntry, content string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if entry.ID == "" {
		entry.ID = generateMemoryID()
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

	// Write content
	if err := fs.writeContent(entry.ID, content); err != nil {
		return err
	}

	fs.index = append(fs.index, entry)
	return fs.saveIndex()
}

// Get retrieves a memory entry and its content by ID.
func (fs *FileStore) Get(id string) (*MemoryEntry, string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	entry := fs.findEntry(id)
	if entry == nil {
		return nil, "", fmt.Errorf("memory %q not found", id)
	}

	content, err := fs.readContent(id)
	if err != nil {
		return nil, "", err
	}

	return entry, content, nil
}

// Update replaces a memory entry's metadata and content.
func (fs *FileStore) Update(entry *MemoryEntry, content string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	idx := fs.findIndex(entry.ID)
	if idx < 0 {
		return fmt.Errorf("memory %q not found", entry.ID)
	}

	entry.UpdatedAt = time.Now()
	fs.index[idx] = entry

	if err := fs.writeContent(entry.ID, content); err != nil {
		return err
	}

	return fs.saveIndex()
}

// Delete removes a memory entry and its content.
func (fs *FileStore) Delete(id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	idx := fs.findIndex(id)
	if idx < 0 {
		return fmt.Errorf("memory %q not found", id)
	}

	// Remove from index
	fs.index = append(fs.index[:idx], fs.index[idx+1:]...)

	// Remove content file
	_ = os.Remove(fs.contentPath(id))

	return fs.saveIndex()
}

// List returns all memory entries.
func (fs *FileStore) List() ([]*MemoryEntry, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	result := make([]*MemoryEntry, len(fs.index))
	copy(result, fs.index)
	return result, nil
}

func (fs *FileStore) findEntry(id string) *MemoryEntry {
	for _, e := range fs.index {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func (fs *FileStore) findIndex(id string) int {
	for i, e := range fs.index {
		if e.ID == id {
			return i
		}
	}
	return -1
}

func (fs *FileStore) indexPath() string {
	return filepath.Join(fs.dir, "index.json")
}

func (fs *FileStore) contentPath(id string) string {
	return filepath.Join(fs.dir, "entries", id+".md")
}

func (fs *FileStore) loadIndex() error {
	data, err := os.ReadFile(fs.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			fs.index = nil
			return nil
		}
		return err
	}

	var entries []*MemoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	fs.index = entries
	return nil
}

func (fs *FileStore) saveIndex() error {
	if err := os.MkdirAll(fs.dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(fs.index, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: tmp + rename
	tmp := fs.indexPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, fs.indexPath())
}

func (fs *FileStore) writeContent(id, content string) error {
	dir := filepath.Join(fs.dir, "entries")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(fs.contentPath(id), []byte(content), 0o644)
}

func (fs *FileStore) readContent(id string) (string, error) {
	data, err := os.ReadFile(fs.contentPath(id))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
