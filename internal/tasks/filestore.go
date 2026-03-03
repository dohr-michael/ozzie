package tasks

import (
	"sort"
	"time"

	"github.com/dohr-michael/ozzie/internal/names"
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

	if t.Name == "" {
		t.Name = names.GenerateUnique(fs.ds.NameExists)
	}

	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now

	dirName := t.ID + "_" + t.Name
	if err := fs.ds.EnsureDir(dirName); err != nil {
		return err
	}

	return fs.ds.WriteMeta(dirName, t)
}

// Get reads task metadata by ID or name.
func (fs *FileStore) Get(ref string) (*Task, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	dir, err := fs.ds.Resolve(ref)
	if err != nil {
		return nil, err
	}

	var t Task
	if err := fs.ds.ReadMeta(dir, &t); err != nil {
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

	dir, err := fs.ds.Resolve(t.ID)
	if err != nil {
		return err
	}

	t.UpdatedAt = time.Now()
	return fs.ds.WriteMeta(dir, t)
}

// Delete removes a task directory.
func (fs *FileStore) Delete(ref string) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	dir, err := fs.ds.Resolve(ref)
	if err != nil {
		return err
	}

	return fs.ds.RemoveDir(dir)
}

// AppendCheckpoint appends a checkpoint entry to the JSONL file.
func (fs *FileStore) AppendCheckpoint(taskID string, cp Checkpoint) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	dir, err := fs.ds.Resolve(taskID)
	if err != nil {
		return err
	}
	return fs.ds.AppendJSONL(dir, "checkpoints.jsonl", cp)
}

// LoadCheckpoints reads all checkpoints from the JSONL file.
func (fs *FileStore) LoadCheckpoints(taskID string) ([]Checkpoint, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	dir, err := fs.ds.Resolve(taskID)
	if err != nil {
		return nil, err
	}
	return dirstore.LoadJSONL[Checkpoint](fs.ds, dir, "checkpoints.jsonl")
}

// WriteOutput writes the task output file.
func (fs *FileStore) WriteOutput(taskID string, content string) error {
	fs.ds.Lock()
	defer fs.ds.Unlock()

	dir, err := fs.ds.Resolve(taskID)
	if err != nil {
		return err
	}
	return fs.ds.WriteFileAtomic(dir, "output.md", []byte(content))
}

// ReadOutput reads the task output file.
func (fs *FileStore) ReadOutput(taskID string) (string, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	dir, err := fs.ds.Resolve(taskID)
	if err != nil {
		return "", err
	}
	data, err := fs.ds.ReadFileContent(dir, "output.md")
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

	dir, err := fs.ds.Resolve(taskID)
	if err != nil {
		return err
	}
	return fs.ds.AppendJSONL(dir, "mailbox.jsonl", msg)
}

// LoadMailbox reads all mailbox messages from the JSONL file.
func (fs *FileStore) LoadMailbox(taskID string) ([]MailboxMessage, error) {
	fs.ds.RLock()
	defer fs.ds.RUnlock()

	dir, err := fs.ds.Resolve(taskID)
	if err != nil {
		return nil, err
	}
	return dirstore.LoadJSONL[MailboxMessage](fs.ds, dir, "mailbox.jsonl")
}
