package sessions

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/storage/dirstore"
)

// FileStore persists sessions as directories with meta.json + messages.jsonl.
type FileStore struct {
	ds *dirstore.DirStore
}

// NewFileStore creates a FileStore rooted at baseDir.
func NewFileStore(baseDir string) *FileStore {
	return &FileStore{ds: dirstore.NewDirStore(baseDir, "session")}
}

func generateSessionID() string {
	u := uuid.New().String()
	return "sess_" + strings.ReplaceAll(u[:8], "-", "")
}

// Create initialises a new session directory with meta.json.
func (fs *FileStore) Create() (*Session, error) {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	now := time.Now()
	s := &Session{
		ID:        generateSessionID(),
		CreatedAt: now,
		UpdatedAt: now,
		Status:    SessionActive,
	}

	if err := fs.ds.EnsureDir(s.ID); err != nil {
		return nil, err
	}

	if err := fs.ds.WriteMeta(s.ID, s); err != nil {
		return nil, err
	}

	return s, nil
}

// Get reads session metadata by ID.
func (fs *FileStore) Get(id string) (*Session, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	var s Session
	if err := fs.ds.ReadMeta(id, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// List returns all sessions sorted by UpdatedAt descending.
func (fs *FileStore) List() ([]*Session, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	dirs, err := fs.ds.ListDirs()
	if err != nil {
		return nil, err
	}

	var sessions []*Session
	for _, name := range dirs {
		var s Session
		if err := fs.ds.ReadMeta(name, &s); err != nil {
			continue // skip corrupted sessions
		}
		sessions = append(sessions, &s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// UpdateMeta atomically rewrites a session's meta.json.
func (fs *FileStore) UpdateMeta(s *Session) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	return fs.ds.WriteMeta(s.ID, s)
}

// Close marks a session as closed.
func (fs *FileStore) Close(id string) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	var s Session
	if err := fs.ds.ReadMeta(id, &s); err != nil {
		return err
	}

	s.Status = SessionClosed
	s.UpdatedAt = time.Now()
	return fs.ds.WriteMeta(s.ID, &s)
}

// AppendMessage appends a message to the session's JSONL file and updates meta.
func (fs *FileStore) AppendMessage(sessionID string, msg Message) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	if err := fs.ds.AppendJSONL(sessionID, "messages.jsonl", msg); err != nil {
		return fmt.Errorf("append message: %w", err)
	}

	var s Session
	if err := fs.ds.ReadMeta(sessionID, &s); err != nil {
		return err
	}
	s.MessageCount++
	s.UpdatedAt = time.Now()
	return fs.ds.WriteMeta(s.ID, &s)
}

// LoadMessages reads all messages from a session's JSONL file.
func (fs *FileStore) LoadMessages(sessionID string) ([]Message, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	return dirstore.LoadJSONL[Message](fs.ds, sessionID, "messages.jsonl")
}
