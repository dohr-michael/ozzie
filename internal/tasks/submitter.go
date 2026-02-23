package tasks

// TaskSubmitter is the interface for submitting and managing tasks.
// Implemented by actors.ActorPool.
type TaskSubmitter interface {
	Submit(t *Task) error
	Cancel(taskID string, reason string) error
	ResumeTask(taskID string) error
	Store() Store
}
