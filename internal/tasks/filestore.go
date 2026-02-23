package tasks

import (
	"sort"
	"time"

	"github.com/dohr-michael/ozzie/internal/storage/dirstore"
)

// FileStore persists tasks as directories with meta.json + checkpoints.jsonl + output.md.
type FileStore struct {
	ds *dirstore.DirStore
}

// NewFileStore creates a FileStore rooted at baseDir.
func NewFileStore(baseDir string) *FileStore {
	return &FileStore{ds: dirstore.NewDirStore(baseDir, "task")}
}

// Create persists a new task to disk.
func (fs *FileStore) Create(t *Task) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	if t.ID == "" {
		t.ID = GenerateTaskID()
	}

	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now

	if err := fs.ds.EnsureDir(t.ID); err != nil {
		return err
	}

	return fs.ds.WriteMeta(t.ID, t)
}

// Get reads task metadata by ID.
func (fs *FileStore) Get(id string) (*Task, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	var t Task
	if err := fs.ds.ReadMeta(id, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// List returns tasks matching the filter, sorted by UpdatedAt descending.
func (fs *FileStore) List(filter ListFilter) ([]*Task, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	dirs, err := fs.ds.ListDirs()
	if err != nil {
		return nil, err
	}

	var tasks []*Task
	for _, name := range dirs {
		var t Task
		if err := fs.ds.ReadMeta(name, &t); err != nil {
			continue // skip corrupted tasks
		}

		// Apply filters
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if filter.SessionID != "" && t.SessionID != filter.SessionID {
			continue
		}
		if filter.ParentID != "" && t.ParentTaskID != filter.ParentID {
			continue
		}

		tasks = append(tasks, &t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].UpdatedAt.After(tasks[j].UpdatedAt)
	})

	return tasks, nil
}

// Update atomically rewrites a task's meta.json.
func (fs *FileStore) Update(t *Task) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	t.UpdatedAt = time.Now()
	return fs.ds.WriteMeta(t.ID, t)
}

// Delete removes a task directory.
func (fs *FileStore) Delete(id string) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	return fs.ds.RemoveDir(id)
}

// AppendCheckpoint appends a checkpoint entry to the JSONL file.
func (fs *FileStore) AppendCheckpoint(taskID string, cp Checkpoint) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	return fs.ds.AppendJSONL(taskID, "checkpoints.jsonl", cp)
}

// LoadCheckpoints reads all checkpoints from the JSONL file.
func (fs *FileStore) LoadCheckpoints(taskID string) ([]Checkpoint, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	return dirstore.LoadJSONL[Checkpoint](fs.ds, taskID, "checkpoints.jsonl")
}

// WriteOutput writes the task output file.
func (fs *FileStore) WriteOutput(taskID string, content string) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	return fs.ds.WriteFileAtomic(taskID, "output.md", []byte(content))
}

// ReadOutput reads the task output file.
func (fs *FileStore) ReadOutput(taskID string) (string, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	data, err := fs.ds.ReadFileContent(taskID, "output.md")
	if err != nil {
		return "", err
	}
	if data == nil {
		return "", nil
	}
	return string(data), nil
}

// AppendMailbox appends a mailbox message to the JSONL file.
func (fs *FileStore) AppendMailbox(taskID string, msg MailboxMessage) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	return fs.ds.AppendJSONL(taskID, "mailbox.jsonl", msg)
}

// LoadMailbox reads all mailbox messages from the JSONL file.
func (fs *FileStore) LoadMailbox(taskID string) ([]MailboxMessage, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	return dirstore.LoadJSONL[MailboxMessage](fs.ds, taskID, "mailbox.jsonl")
}
