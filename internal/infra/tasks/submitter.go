package tasks

import "github.com/dohr-michael/ozzie/internal/core/brain"

// TaskSubmitter is the interface for submitting and managing tasks.
// Canonical definition lives in brain.TaskSubmitter.
type TaskSubmitter = brain.TaskSubmitter

// InlineExecutor can execute tasks synchronously when the pool has 1 actor.
// Canonical definition lives in brain.InlineExecutor.
type InlineExecutor = brain.InlineExecutor
