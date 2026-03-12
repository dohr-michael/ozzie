// Package tasks provides persistent task management for async agent execution.
package tasks

import "github.com/dohr-michael/ozzie/internal/brain"

// Type aliases — canonical definitions live in brain/ (domain layer).

type TaskStatus = brain.TaskStatus

const (
	TaskPending   = brain.TaskPending
	TaskRunning   = brain.TaskRunning
	TaskCompleted = brain.TaskCompleted
	TaskFailed    = brain.TaskFailed
	TaskCancelled = brain.TaskCancelled
)

type TaskPriority = brain.TaskPriority

const (
	PriorityLow    = brain.PriorityLow
	PriorityNormal = brain.PriorityNormal
	PriorityHigh   = brain.PriorityHigh
)

type TaskProgress = brain.TaskProgress
type TaskConfig = brain.TaskConfig
type TokenUsage = brain.TokenUsage
type TaskResult = brain.TaskResult
type Task = brain.Task
type Checkpoint = brain.Checkpoint
type ListFilter = brain.ListFilter
type ActorInfo = brain.ActorInfo
