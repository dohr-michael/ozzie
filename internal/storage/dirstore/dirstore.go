package dirstore

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DirStore provides common primitives for directory-based file stores.
// Each entity gets its own subdirectory with a meta.json + optional companion files.
type DirStore struct {
	mu         sync.RWMutex
	baseDir    string
	entityName string // for error messages: "session", "task"
}

// NewDirStore creates a DirStore rooted at baseDir.
func NewDirStore(baseDir, entityName string) *DirStore {
	return &DirStore{baseDir: baseDir, entityName: entityName}
}

// Lock acquires an exclusive lock.
func (ds *DirStore) Lock() { ds.mu.Lock() }

// Unlock releases an exclusive lock.
func (ds *DirStore) Unlock() { ds.mu.Unlock() }

// RLock acquires a shared read lock.
func (ds *DirStore) RLock() { ds.mu.RLock() }

// RUnlock releases a shared read lock.
func (ds *DirStore) RUnlock() { ds.mu.RUnlock() }

// Dir returns the directory path for a given entity ID.
func (ds *DirStore) Dir(id string) string {
	return filepath.Join(ds.baseDir, id)
}

// FilePath returns the path to a named file within an entity's directory.
func (ds *DirStore) FilePath(id, name string) string {
	return filepath.Join(ds.baseDir, id, name)
}

// EnsureDir creates the entity directory (and parents) if it doesn't exist.
func (ds *DirStore) EnsureDir(id string) error {
	if err := os.MkdirAll(ds.Dir(id), 0o755); err != nil {
		return fmt.Errorf("create %s dir: %w", ds.entityName, err)
	}
	return nil
}

// RemoveDir removes the entity directory and all its contents.
func (ds *DirStore) RemoveDir(id string) error {
	return os.RemoveAll(ds.Dir(id))
}

// ListDirs returns the names of all subdirectories in baseDir.
func (ds *DirStore) ListDirs() ([]string, error) {
	entries, err := os.ReadDir(ds.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list %ss dir: %w", ds.entityName, err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// WriteMeta atomically writes meta.json using a temp file + rename.
func (ds *DirStore) WriteMeta(id string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	path := ds.FilePath(id, "meta.json")
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write meta tmp: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename meta: %w", err)
	}

	return nil
}

// ReadMeta reads and unmarshals meta.json into out.
func (ds *DirStore) ReadMeta(id string, out any) error {
	data, err := os.ReadFile(ds.FilePath(id, "meta.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found: %s", ds.entityName, id)
		}
		return fmt.Errorf("read meta: %w", err)
	}

	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unmarshal meta: %w", err)
	}

	return nil
}

// AppendJSONL appends a JSON-encoded line to the given file within an entity's directory.
func (ds *DirStore) AppendJSONL(id, filename string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filename, err)
	}

	f, err := os.OpenFile(ds.FilePath(id, filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", filename, err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}

	return nil
}

// LoadJSONL reads all JSON lines from a file, deserializing each into type T.
func LoadJSONL[T any](ds *DirStore, id, filename string) ([]T, error) {
	f, err := os.Open(ds.FilePath(id, filename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", filename, err)
	}
	defer f.Close()

	var items []T
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var item T
		if err := json.Unmarshal(line, &item); err != nil {
			continue // skip corrupted lines
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", filename, err)
	}

	return items, nil
}

// WriteFileAtomic atomically writes content to a named file using tmp + rename.
func (ds *DirStore) WriteFileAtomic(id, filename string, content []byte) error {
	path := ds.FilePath(id, filename)
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return fmt.Errorf("write %s tmp: %w", filename, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s: %w", filename, err)
	}

	return nil
}

// ReadFileContent reads the content of a named file. Returns nil, nil if the file doesn't exist.
func (ds *DirStore) ReadFileContent(id, filename string) ([]byte, error) {
	data, err := os.ReadFile(ds.FilePath(id, filename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", filename, err)
	}
	return data, nil
}
