package tasks

// ListFilter defines criteria for filtering task lists.
type ListFilter struct {
	Status   TaskStatus `json:"status,omitempty"`
	SessionID string   `json:"session_id,omitempty"`
	ParentID  string   `json:"parent_id,omitempty"`
}

// Store defines the persistence interface for tasks.
type Store interface {
	Create(t *Task) error
	Get(id string) (*Task, error)
	List(filter ListFilter) ([]*Task, error)
	Update(t *Task) error
	Delete(id string) error
	AppendCheckpoint(taskID string, cp Checkpoint) error
	LoadCheckpoints(taskID string) ([]Checkpoint, error)
	WriteOutput(taskID string, content string) error
	ReadOutput(taskID string) (string, error)
	AppendMailbox(taskID string, msg MailboxMessage) error
	LoadMailbox(taskID string) ([]MailboxMessage, error)
}
