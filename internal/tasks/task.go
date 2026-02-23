// Package tasks provides persistent task management for async agent execution.
package tasks

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/sessions"
)

// AutonomyLevel controls how much independence a task has.
const (
	AutonomyDisabled   = "disabled"   // standard single-step execution
	AutonomySupervised = "supervised" // explore → plan → validate → execute
	AutonomyAutonomous = "autonomous" // explore → plan → execute (no validation)
)

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
	TaskSuspended TaskStatus = "suspended"
)

// TaskPriority represents the execution priority of a task.
type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityNormal TaskPriority = "normal"
	PriorityHigh   TaskPriority = "high"
)

// TaskProgress tracks step-level progress within a task.
type TaskProgress struct {
	CurrentStep      int    `json:"current_step"`
	TotalSteps       int    `json:"total_steps"`
	CurrentStepLabel string `json:"current_step_label,omitempty"`
	Percentage       int    `json:"percentage"`
}

// TaskPlanStep is a single step in a task plan.
type TaskPlanStep struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
}

// TaskPlan is the decomposed execution plan for a task.
type TaskPlan struct {
	Steps []TaskPlanStep `json:"steps"`
}

// TaskConfig holds execution parameters for a task.
type TaskConfig struct {
	Model         string            `json:"model,omitempty"`
	Tools         []string          `json:"tools,omitempty"`
	Skill         string            `json:"skill,omitempty"`
	WorkDir       string            `json:"work_dir,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	AutonomyLevel string            `json:"autonomy_level,omitempty"` // "disabled" | "supervised" | "autonomous"
}

// IsCoordinator returns true if the task uses any coordinator workflow (supervised or autonomous).
func (c TaskConfig) IsCoordinator() bool {
	return c.AutonomyLevel == AutonomySupervised || c.AutonomyLevel == AutonomyAutonomous
}

// UnmarshalJSON handles backward compatibility with the old "coordinator" bool field.
func (c *TaskConfig) UnmarshalJSON(data []byte) error {
	type Alias TaskConfig
	aux := &struct {
		Coordinator *bool `json:"coordinator,omitempty"`
		*Alias
	}{Alias: (*Alias)(c)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if c.AutonomyLevel == "" && aux.Coordinator != nil && *aux.Coordinator {
		c.AutonomyLevel = AutonomySupervised
	}
	return nil
}

// TaskResult holds the outcome of a completed task.
type TaskResult struct {
	OutputPath string              `json:"output_path,omitempty"`
	Error      string              `json:"error,omitempty"`
	TokenUsage sessions.TokenUsage `json:"token_usage"`
}

// Task represents an async unit of work.
type Task struct {
	ID              string       `json:"id"`
	SessionID       string       `json:"session_id,omitempty"`
	ParentTaskID    string       `json:"parent_task_id,omitempty"`
	DependsOn       []string     `json:"depends_on,omitempty"`
	Title           string       `json:"title"`
	Description     string       `json:"description"`
	Status          TaskStatus   `json:"status"`
	Priority        TaskPriority `json:"priority"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	StartedAt       *time.Time   `json:"started_at,omitempty"`
	CompletedAt     *time.Time   `json:"completed_at,omitempty"`
	Progress        TaskProgress `json:"progress"`
	Plan            *TaskPlan    `json:"plan,omitempty"`
	Config          TaskConfig   `json:"config"`
	Result          *TaskResult  `json:"result,omitempty"`
	Tags            []string     `json:"tags,omitempty"`
	SuspendedAt     *time.Time   `json:"suspended_at,omitempty"`
	SuspendCount    int          `json:"suspend_count"`
	RetryCount      int          `json:"retry_count"`
	MaxRetries      int          `json:"max_retries"`
	WaitingForReply bool         `json:"waiting_for_reply,omitempty"`
}

// MailboxMessage represents a message in a task's mailbox (request/response pairs).
type MailboxMessage struct {
	ID        string    `json:"id"`
	Ts        time.Time `json:"ts"`
	Type      string    `json:"type"`                 // "request" | "response" | "exploration"
	Token     string    `json:"token"`                // links request → response
	Content   string    `json:"content"`              // plan (request) or feedback (response)
	Status    string    `json:"status,omitempty"`     // "approved" | "revise" (response only)
	SessionID string    `json:"session_id,omitempty"` // originating session
}

// Checkpoint records a point-in-time snapshot of task progress.
type Checkpoint struct {
	Ts      time.Time `json:"ts"`
	StepID  string    `json:"step_id,omitempty"`
	Type    string    `json:"type"`
	Summary string    `json:"summary"`
}

// GenerateTaskID creates a unique task identifier.
func GenerateTaskID() string {
	u := uuid.New().String()
	return "task_" + strings.ReplaceAll(u[:8], "-", "")
}
