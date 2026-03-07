package tasks

import "context"

// ActorInfo describes one available actor for task scheduling.
type ActorInfo struct {
	ProviderName string   `json:"provider_name"`
	Tags         []string `json:"tags,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// TaskSubmitter is the interface for submitting and managing tasks.
// Implemented by actors.ActorPool.
type TaskSubmitter interface {
	Submit(t *Task) error
	Cancel(taskID string, reason string) error
	Store() Store
	// AvailableActors returns a deduplicated summary of actor tags and capabilities.
	AvailableActors() []ActorInfo
}

// InlineExecutor is implemented by pools that can execute tasks synchronously
// in the caller's goroutine. When the pool has only one actor, async submission
// would deadlock (the single actor is occupied by the parent agent). Inline
// execution avoids this by running the child task directly.
//
// TODO(solution-2): Multi-actor contention — when all actors are busy but pool
// size > 1, the parent task should yield/suspend its actor slot (via checkpoint),
// let the child run, then resume when the child completes.
// See: heartbeat + checkpoint system in runner.go.
type InlineExecutor interface {
	ShouldInline() bool
	ExecuteInline(ctx context.Context, t *Task) (output string, err error)
}
