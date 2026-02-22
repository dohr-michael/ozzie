package sessions

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FileStore persists sessions as directories with meta.json + messages.jsonl.
type FileStore struct {
	mu      sync.RWMutex
	baseDir string
}

// NewFileStore creates a FileStore rooted at baseDir.
func NewFileStore(baseDir string) *FileStore {
	return &FileStore{baseDir: baseDir}
}

func (fs *FileStore) sessionDir(id string) string {
	return filepath.Join(fs.baseDir, id)
}

func (fs *FileStore) metaPath(id string) string {
	return filepath.Join(fs.sessionDir(id), "meta.json")
}

func (fs *FileStore) messagesPath(id string) string {
	return filepath.Join(fs.sessionDir(id), "messages.jsonl")
}

func generateSessionID() string {
	u := uuid.New().String()
	return "sess_" + strings.ReplaceAll(u[:8], "-", "")
}

// Create initialises a new session directory with meta.json.
func (fs *FileStore) Create() (*Session, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	now := time.Now()
	s := &Session{
		ID:        generateSessionID(),
		CreatedAt: now,
		UpdatedAt: now,
		Status:    SessionActive,
	}

	dir := fs.sessionDir(s.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	if err := fs.writeMeta(s); err != nil {
		return nil, err
	}

	return s, nil
}

// Get reads session metadata by ID.
func (fs *FileStore) Get(id string) (*Session, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	return fs.readMeta(id)
}

// List returns all sessions sorted by UpdatedAt descending.
func (fs *FileStore) List() ([]*Session, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	entries, err := os.ReadDir(fs.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list sessions dir: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		s, err := fs.readMeta(entry.Name())
		if err != nil {
			continue // skip corrupted sessions
		}
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// UpdateMeta atomically rewrites a session's meta.json.
func (fs *FileStore) UpdateMeta(s *Session) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.writeMeta(s)
}

// Close marks a session as closed.
func (fs *FileStore) Close(id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	s, err := fs.readMeta(id)
	if err != nil {
		return err
	}

	s.Status = SessionClosed
	s.UpdatedAt = time.Now()
	return fs.writeMeta(s)
}

// AppendMessage appends a message to the session's JSONL file and updates meta.
func (fs *FileStore) AppendMessage(sessionID string, msg Message) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	f, err := os.OpenFile(fs.messagesPath(sessionID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open messages file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write message: %w", err)
	}

	// Update meta
	s, err := fs.readMeta(sessionID)
	if err != nil {
		return err
	}
	s.MessageCount++
	s.UpdatedAt = time.Now()
	return fs.writeMeta(s)
}

// LoadMessages reads all messages from a session's JSONL file.
func (fs *FileStore) LoadMessages(sessionID string) ([]Message, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	return fs.loadMessages(sessionID)
}

func (fs *FileStore) loadMessages(sessionID string) ([]Message, error) {
	f, err := os.Open(fs.messagesPath(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open messages file: %w", err)
	}
	defer f.Close()

	var messages []Message
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			continue // skip corrupted lines
		}
		messages = append(messages, msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan messages: %w", err)
	}

	return messages, nil
}

// writeMeta atomically writes meta.json using a temp file + rename.
func (fs *FileStore) writeMeta(s *Session) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	path := fs.metaPath(s.ID)
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write meta tmp: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename meta: %w", err)
	}

	return nil
}

// readMeta reads a session's meta.json.
func (fs *FileStore) readMeta(id string) (*Session, error) {
	data, err := os.ReadFile(fs.metaPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("read meta: %w", err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal meta: %w", err)
	}

	return &s, nil
}
