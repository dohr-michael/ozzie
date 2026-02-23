package plugins

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/tasks"
)

// =============================================================================
// submit_task
// =============================================================================

// SubmitTaskTool submits a new async task to the actor pool.
type SubmitTaskTool struct {
	pool            tasks.TaskSubmitter
	autonomyDefault string // "disabled" | "supervised" | "autonomous"
}

// NewSubmitTaskTool creates a new submit_task tool.
func NewSubmitTaskTool(pool tasks.TaskSubmitter, autonomyDefault string) *SubmitTaskTool {
	return &SubmitTaskTool{pool: pool, autonomyDefault: autonomyDefault}
}

// SubmitTaskManifest returns the plugin manifest for the submit_task tool.
func SubmitTaskManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "submit_task",
		Description: "Submit an async task for background execution",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "submit_task",
				Description: "Submit a task for asynchronous background execution by a sub-agent. Returns the task ID immediately. Use check_task to monitor progress.",
				Parameters: map[string]ParamSpec{
					"title": {
						Type:        "string",
						Description: "Short title for the task",
						Required:    true,
					},
					"description": {
						Type:        "string",
						Description: "Detailed description of what the task should accomplish",
						Required:    true,
					},
					"tools": {
						Type:        "array",
						Description: "List of tool names the task agent can use. Defaults to [\"run_command\", \"git\"] if omitted.",
					},
					"priority": {
						Type:        "string",
						Description: "Task priority: low, normal, or high",
						Enum:        []string{"low", "normal", "high"},
					},
					"work_dir": {
						Type:        "string",
						Description: "Absolute path to the working directory. Commands and file operations will use this as their base directory.",
					},
					"env": {
						Type:        "object",
						Description: "Additional environment variables for the task. Example: {\"PROJECT_NAME\": \"chess\"}",
					},
					"depends_on": {
						Type:        "array",
						Description: "Task IDs that must complete before this task starts",
					},
					"autonomy_level": {
						Type:        "string",
						Description: "Autonomy level: 'disabled' (standard single-step), 'supervised' (explore → plan → validate → execute), 'autonomous' (explore → plan → execute without validation). Defaults to system setting.",
						Enum:        []string{"disabled", "supervised", "autonomous"},
					},
					"skill": {
						Type:        "string",
						Description: "Name of a skill to execute directly (bypasses agent reasoning)",
					},
				},
			},
		},
	}
}

type submitTaskInput struct {
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Tools         []string          `json:"tools"`
	WorkDir       string            `json:"work_dir,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Priority      string            `json:"priority"`
	DependsOn     []string          `json:"depends_on"`
	AutonomyLevel string            `json:"autonomy_level,omitempty"` // empty = use system default
	Skill         string            `json:"skill,omitempty"`
}

// Info returns the tool info for Eino registration.
func (t *SubmitTaskTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&SubmitTaskManifest().Tools[0]), nil
}

// InvokableRun submits a task and returns the task ID.
func (t *SubmitTaskTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input submitTaskInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("submit_task: parse input: %w", err)
	}
	if input.Title == "" {
		return "", fmt.Errorf("submit_task: title is required")
	}
	if input.Description == "" {
		return "", fmt.Errorf("submit_task: description is required")
	}

	priority := tasks.PriorityNormal
	if input.Priority != "" {
		priority = tasks.TaskPriority(input.Priority)
	}

	sessionID := events.SessionIDFromContext(ctx)

	// Resolve autonomy level: explicit per-task, or system default
	autonomy := t.autonomyDefault
	if input.AutonomyLevel != "" {
		autonomy = input.AutonomyLevel
	}

	tools := input.Tools
	// Default tools: if none specified and not a skill task, provide base action tools
	if len(tools) == 0 && input.Skill == "" {
		tools = []string{"run_command", "git"}
	}
	// Supervised mode: ensure request_validation is in the tool set
	if autonomy == tasks.AutonomySupervised && !containsStr(tools, "request_validation") {
		tools = append(tools, "request_validation")
	}

	task := &tasks.Task{
		SessionID:   sessionID,
		Title:       input.Title,
		Description: input.Description,
		Priority:    priority,
		DependsOn:   input.DependsOn,
		Config: tasks.TaskConfig{
			Tools:         tools,
			WorkDir:       input.WorkDir,
			Env:           input.Env,
			Skill:         input.Skill,
			AutonomyLevel: autonomy,
		},
	}

	if err := t.pool.Submit(task); err != nil {
		return "", fmt.Errorf("submit_task: %w", err)
	}

	result, _ := json.Marshal(map[string]string{
		"task_id": task.ID,
		"status":  "submitted",
	})
	return string(result), nil
}

var _ tool.InvokableTool = (*SubmitTaskTool)(nil)

// =============================================================================
// check_task
// =============================================================================

// CheckTaskTool retrieves the status and progress of a task.
type CheckTaskTool struct {
	store tasks.Store
}

// NewCheckTaskTool creates a new check_task tool.
func NewCheckTaskTool(store tasks.Store) *CheckTaskTool {
	return &CheckTaskTool{store: store}
}

// CheckTaskManifest returns the plugin manifest for the check_task tool.
func CheckTaskManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "check_task",
		Description: "Check the status and progress of a submitted task",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "check_task",
				Description: "Check the status, progress, and output of an async task by its ID.",
				Parameters: map[string]ParamSpec{
					"task_id": {
						Type:        "string",
						Description: "The task ID to check",
						Required:    true,
					},
				},
			},
		},
	}
}

type checkTaskInput struct {
	TaskID string `json:"task_id"`
}

type checkTaskOutput struct {
	ID       string             `json:"id"`
	Title    string             `json:"title"`
	Status   tasks.TaskStatus   `json:"status"`
	Progress tasks.TaskProgress `json:"progress"`
	Output   string             `json:"output,omitempty"`
	Error    string             `json:"error,omitempty"`
}

// Info returns the tool info for Eino registration.
func (t *CheckTaskTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&CheckTaskManifest().Tools[0]), nil
}

// InvokableRun checks a task's status and returns it.
func (t *CheckTaskTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input checkTaskInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("check_task: parse input: %w", err)
	}
	if input.TaskID == "" {
		return "", fmt.Errorf("check_task: task_id is required")
	}

	task, err := t.store.Get(input.TaskID)
	if err != nil {
		return "", fmt.Errorf("check_task: %w", err)
	}

	out := checkTaskOutput{
		ID:       task.ID,
		Title:    task.Title,
		Status:   task.Status,
		Progress: task.Progress,
	}

	if task.Status == tasks.TaskCompleted {
		output, _ := t.store.ReadOutput(task.ID)
		if len(output) > 500 {
			output = output[:500] + "..."
		}
		out.Output = output
	}

	if task.Result != nil && task.Result.Error != "" {
		out.Error = task.Result.Error
	}

	result, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("check_task: marshal: %w", err)
	}
	return string(result), nil
}

var _ tool.InvokableTool = (*CheckTaskTool)(nil)

// =============================================================================
// cancel_task
// =============================================================================

// CancelTaskTool cancels a running or pending task.
type CancelTaskTool struct {
	pool tasks.TaskSubmitter
}

// NewCancelTaskTool creates a new cancel_task tool.
func NewCancelTaskTool(pool tasks.TaskSubmitter) *CancelTaskTool {
	return &CancelTaskTool{pool: pool}
}

// CancelTaskManifest returns the plugin manifest for the cancel_task tool.
func CancelTaskManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "cancel_task",
		Description: "Cancel a running or pending task",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "cancel_task",
				Description: "Cancel an async task by its ID. Running tasks will be interrupted.",
				Parameters: map[string]ParamSpec{
					"task_id": {
						Type:        "string",
						Description: "The task ID to cancel",
						Required:    true,
					},
					"reason": {
						Type:        "string",
						Description: "Optional reason for cancellation",
					},
				},
			},
		},
	}
}

type cancelTaskInput struct {
	TaskID string `json:"task_id"`
	Reason string `json:"reason"`
}

// Info returns the tool info for Eino registration.
func (t *CancelTaskTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&CancelTaskManifest().Tools[0]), nil
}

// InvokableRun cancels a task.
func (t *CancelTaskTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input cancelTaskInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("cancel_task: parse input: %w", err)
	}
	if input.TaskID == "" {
		return "", fmt.Errorf("cancel_task: task_id is required")
	}

	reason := input.Reason
	if reason == "" {
		reason = "cancelled by user"
	}

	if err := t.pool.Cancel(input.TaskID, reason); err != nil {
		return "", fmt.Errorf("cancel_task: %w", err)
	}

	result, _ := json.Marshal(map[string]string{
		"task_id": input.TaskID,
		"status":  "cancelled",
	})
	return string(result), nil
}

var _ tool.InvokableTool = (*CancelTaskTool)(nil)

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
