package hands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/brain/conscience"
	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/dohr-michael/ozzie/internal/tasks"
)

// =============================================================================
// submit_task
// =============================================================================

// SubmitTaskTool submits a new async task to the actor pool.
type SubmitTaskTool struct {
	pool     tasks.TaskSubmitter
	registry *ToolRegistry    // for looking up tool specs (dangerous flag)
	perms    *conscience.ToolPermissions // for checking/setting approvals
	bus      events.EventBus  // for emitting approval prompts
}

// NewSubmitTaskTool creates a new submit_task tool.
func NewSubmitTaskTool(pool tasks.TaskSubmitter, registry *ToolRegistry, perms *conscience.ToolPermissions, bus events.EventBus) *SubmitTaskTool {
	return &SubmitTaskTool{
		pool:     pool,
		registry: registry,
		perms:    perms,
		bus:      bus,
	}
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
				Description: "Submit a task for asynchronous background execution by a sub-agent. Returns the task ID immediately. Use check_task to monitor progress. Prefer this over plan_task for single tasks or simple sequential work.",
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
					"skill": {
						Type:        "string",
						Description: "Name of a skill to execute directly (bypasses agent reasoning)",
					},
					"actor_tags": {
						Type:        "array",
						Description: "Tags to match actors (e.g. [\"self-hosted\"]). The task will run on an actor that has ALL specified tags.",
					},
					"required_capabilities": {
						Type:        "array",
						Description: "Required model capabilities (e.g. [\"coding\", \"tool_use\"]). The task will run on an actor whose model supports ALL specified capabilities.",
					},
					"tool_constraints": {
						Type:        "object",
						Description: "Per-tool argument constraints. Map of tool name to constraint object with fields: allowed_commands, allowed_patterns, blocked_patterns, allowed_paths, allowed_domains.",
					},
				},
			},
		},
	}
}

type submitTaskInput struct {
	Title                string                            `json:"title"`
	Description          string                            `json:"description"`
	Tools                []string                          `json:"tools"`
	WorkDir              string                            `json:"work_dir,omitempty"`
	Env                  map[string]string                 `json:"env,omitempty"`
	Priority             string                            `json:"priority"`
	DependsOn            []string                          `json:"depends_on"`
	Skill                string                            `json:"skill,omitempty"`
	ActorTags            []string                          `json:"actor_tags,omitempty"`
	RequiredCapabilities []string                          `json:"required_capabilities,omitempty"`
	ToolConstraints      map[string]*events.ToolConstraint `json:"tool_constraints,omitempty"`
}

// Info returns the tool info for Eino registration.
// Descriptions for actor_tags and required_capabilities are dynamically enriched
// with available actors from the pool, so the LLM knows what to target.
func (t *SubmitTaskTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	manifest := SubmitTaskManifest()
	spec := &manifest.Tools[0]
	enrichActorParamDescriptions(spec, t.pool.AvailableActors())
	return toolSpecToToolInfo(spec), nil
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

	tools := input.Tools
	// Default tools: if none specified and not a skill task, provide base action tools
	if len(tools) == 0 && input.Skill == "" {
		tools = DefaultTaskTools
	}

	// Pre-approve dangerous tools before submitting
	if t.registry != nil && t.perms != nil && t.bus != nil {
		if err := t.preApproveDangerousTools(ctx, sessionID, tools); err != nil {
			return "", fmt.Errorf("submit_task: %w", err)
		}
	}

	// Inherit parent session's WorkDir if not explicitly set
	workDir := input.WorkDir
	if workDir == "" {
		workDir = events.WorkDirFromContext(ctx)
	}

	// Resolve relative work_dir to absolute so sub-agents find the directory
	workDir, err := resolveAbsWorkDir(workDir, "submit_task")
	if err != nil {
		return "", err
	}

	// Merge tool constraints: session constraints + task-specific (intersection)
	sessionConstraints := events.ToolConstraintsFromContext(ctx)
	taskConstraints := events.MergeToolConstraints(sessionConstraints, input.ToolConstraints)

	task := &tasks.Task{
		SessionID:   sessionID,
		Title:       input.Title,
		Description: input.Description,
		Priority:    priority,
		DependsOn:   input.DependsOn,
		Tags:        input.ActorTags,
		Config: tasks.TaskConfig{
			Tools:                tools,
			WorkDir:              workDir,
			Env:                  input.Env,
			Skill:                input.Skill,
			RequiredTags:         input.ActorTags,
			RequiredCapabilities: input.RequiredCapabilities,
			ToolConstraints:      taskConstraints,
		},
	}

	// Inline execution: when the pool has a single actor, async submission
	// would deadlock. Execute synchronously instead.
	if inliner, ok := t.pool.(tasks.InlineExecutor); ok && inliner.ShouldInline() {
		output, err := inliner.ExecuteInline(ctx, task)
		if err != nil {
			result, _ := json.Marshal(map[string]any{
				"task_id": task.ID,
				"status":  "failed",
				"error":   err.Error(),
			})
			return string(result), nil
		}
		if len(output) > 2000 {
			output = output[:2000] + "..."
		}
		result, _ := json.Marshal(map[string]any{
			"task_id": task.ID,
			"status":  "completed",
			"output":  output,
		})
		return string(result), nil
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

// preApproveDangerousTools checks if any tools in the list are dangerous and
// not yet approved. If so, prompts the user for batch approval before submit.
func (t *SubmitTaskTool) preApproveDangerousTools(ctx context.Context, sessionID string, toolNames []string) error {
	var unapproved []string
	for _, name := range toolNames {
		spec := t.registry.ToolSpec(name)
		if spec == nil || !spec.Dangerous {
			continue
		}
		if t.perms.IsAllowed(sessionID, name) {
			continue
		}
		unapproved = append(unapproved, name)
	}
	if len(unapproved) == 0 {
		return nil
	}

	return conscience.PromptToolApproval(ctx, t.bus, t.perms, sessionID, unapproved,
		"Task requires dangerous tools: %s. Allow?")
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
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	Status       tasks.TaskStatus   `json:"status"`
	Progress     tasks.TaskProgress `json:"progress"`
	ActorID      string             `json:"actor_id,omitempty"`
	ProviderName string             `json:"provider_name,omitempty"`
	Output       string             `json:"output,omitempty"`
	Error        string             `json:"error,omitempty"`
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
		ID:           task.ID,
		Title:        task.Title,
		Status:       task.Status,
		Progress:     task.Progress,
		ActorID:      task.ActorID,
		ProviderName: task.ProviderName,
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
