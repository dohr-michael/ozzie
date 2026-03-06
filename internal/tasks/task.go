// Package tasks provides persistent task management for async agent execution.
package tasks

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/sessions"
)

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
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

// TaskConfig holds execution parameters for a task.
type TaskConfig struct {
	Model                string            `json:"model,omitempty"`
	Tools                []string          `json:"tools,omitempty"`
	Skill                string            `json:"skill,omitempty"`
	WorkDir              string            `json:"work_dir,omitempty"`
	Env                  map[string]string `json:"env,omitempty"`
	RequiredTags         []string          `json:"required_tags,omitempty"`
	RequiredCapabilities []string          `json:"required_capabilities,omitempty"`
	ApprovedTools        []string          `json:"approved_tools,omitempty"` // dangerous tools pre-approved
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
	Name            string       `json:"name,omitempty"`
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
	Progress   TaskProgress `json:"progress"`
	Config     TaskConfig   `json:"config"`
	Result     *TaskResult  `json:"result,omitempty"`
	Tags       []string     `json:"tags,omitempty"`
	RetryCount int          `json:"retry_count"`
	MaxRetries int          `json:"max_retries"`
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
