package hands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/core/conscience"
	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/dohr-michael/ozzie/internal/infra/tasks"
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
		Description: "Submit an async task or multi-step plan for background execution",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "submit_task",
				Description: "Submit a task for asynchronous background execution by a sub-agent. Returns the task ID immediately. Use query_tasks to monitor progress. For multi-step plans with dependencies, provide steps[] instead of title/description.",
				Parameters: map[string]ParamSpec{
					"title": {
						Type:        "string",
						Description: "Short title for the task (required for single task, optional plan title for steps)",
						Required:    true,
					},
					"description": {
						Type:        "string",
						Description: "Detailed description of what the task should accomplish (required for single task, ignored when steps is provided)",
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
					"steps": {
						Type:        "array",
						Description: "Multi-step plan: ordered list of steps with dependencies. Steps with no depends_on run in parallel. When provided, this creates multiple sub-tasks instead of a single task.",
						Items: &ParamSpec{
							Type: "object",
							Properties: map[string]ParamSpec{
								"title": {
									Type:        "string",
									Description: "Short title for this step",
									Required:    true,
								},
								"description": {
									Type:        "string",
									Description: "Detailed description of what this step should accomplish",
									Required:    true,
								},
								"tools": {
									Type:        "array",
									Description: "Tool names this step is allowed to use",
									Items:       &ParamSpec{Type: "string"},
								},
								"depends_on": {
									Type:        "array",
									Description: "Indices (0-based) of steps that must complete before this step can start.",
									Items:       &ParamSpec{Type: "integer"},
								},
								"actor_tags": {
									Type:        "array",
									Description: "Tags to match actors for this step.",
									Items:       &ParamSpec{Type: "string"},
								},
								"required_capabilities": {
									Type:        "array",
									Description: "Required model capabilities for this step.",
									Items:       &ParamSpec{Type: "string"},
								},
							},
						},
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
	Steps                []planStep                        `json:"steps,omitempty"`
}

// planStep represents a single step in a multi-step plan.
type planStep struct {
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	Tools                []string `json:"tools,omitempty"`
	DependsOn            []int    `json:"depends_on,omitempty"`
	ActorTags            []string `json:"actor_tags,omitempty"`
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
}

// Info returns the tool info for Eino registration.
// Descriptions for actor_tags and required_capabilities are dynamically enriched
// with available actors from the pool, so the LLM knows what to target.
func (t *SubmitTaskTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	manifest := SubmitTaskManifest()
	spec := &manifest.Tools[0]
	actors := t.pool.AvailableActors()
	enrichActorParamDescriptions(spec, actors)
	// Also enrich step-level actor params
	if stepsParam, ok := spec.Parameters["steps"]; ok && stepsParam.Items != nil {
		enrichActorParamDescriptions(&ToolSpec{Parameters: stepsParam.Items.Properties}, actors)
	}
	return toolSpecToToolInfo(spec), nil
}

// InvokableRun submits a task (or multi-step plan) and returns the task ID(s).
func (t *SubmitTaskTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input submitTaskInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("submit_task: parse input: %w", err)
	}
	if input.Title == "" {
		return "", fmt.Errorf("submit_task: title is required")
	}

	// Multi-step plan mode
	if len(input.Steps) > 0 {
		return t.runPlan(ctx, input)
	}

	if input.Description == "" {
		return "", fmt.Errorf("submit_task: description is required")
	}

	return t.runSingle(ctx, input)
}

// runSingle submits a single task.
func (t *SubmitTaskTool) runSingle(ctx context.Context, input submitTaskInput) (string, error) {
	priority := tasks.PriorityNormal
	if input.Priority != "" {
		priority = tasks.TaskPriority(input.Priority)
	}

	sessionID := events.SessionIDFromContext(ctx)

	tools := input.Tools
	if len(tools) == 0 && input.Skill == "" {
		tools = DefaultTaskTools
	}

	if t.registry != nil && t.perms != nil && t.bus != nil {
		if err := t.preApproveDangerousTools(ctx, sessionID, tools); err != nil {
			return "", fmt.Errorf("submit_task: %w", err)
		}
	}

	workDir := input.WorkDir
	if workDir == "" {
		workDir = events.WorkDirFromContext(ctx)
	}

	workDir, err := resolveAbsWorkDir(workDir, "submit_task")
	if err != nil {
		return "", err
	}

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

// inlinePlanResult is the JSON shape returned by inline plan execution.
type inlinePlanResult struct {
	PlanID string            `json:"plan_id"`
	Title  string            `json:"title"`
	Status string            `json:"status"`
	Tasks  []inlinePlanEntry `json:"tasks"`
}

type inlinePlanEntry struct {
	Step   int    `json:"step"`
	TaskID string `json:"task_id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// planTaskResult is the JSON shape for async plan submissions.
type planTaskResult struct {
	PlanID string          `json:"plan_id"`
	Title  string          `json:"title"`
	Tasks  []planTaskEntry `json:"tasks"`
}

type planTaskEntry struct {
	Step   int    `json:"step"`
	TaskID string `json:"task_id"`
	Title  string `json:"title"`
}

// runPlan creates a multi-step plan with dependent sub-tasks.
func (t *SubmitTaskTool) runPlan(ctx context.Context, input submitTaskInput) (string, error) {
	// Validate steps
	for i, step := range input.Steps {
		for _, dep := range step.DependsOn {
			if dep < 0 || dep >= i {
				return "", fmt.Errorf("submit_task: step %d has invalid depends_on index %d (must be 0..%d)", i, dep, i-1)
			}
		}
	}

	resolved, err := resolveAbsWorkDir(input.WorkDir, "submit_task")
	if err != nil {
		return "", err
	}
	input.WorkDir = resolved

	sessionID := events.SessionIDFromContext(ctx)

	// Inline execution for single-actor pools
	if inliner, ok := t.pool.(tasks.InlineExecutor); ok && inliner.ShouldInline() {
		return t.runPlanInline(ctx, inliner, input, sessionID)
	}

	return t.runPlanAsync(input, sessionID)
}

func (t *SubmitTaskTool) runPlanInline(ctx context.Context, inliner tasks.InlineExecutor, input submitTaskInput, sessionID string) (string, error) {
	taskIDs := make([]string, len(input.Steps))
	results := make([]inlinePlanEntry, len(input.Steps))
	overallStatus := "completed"

	for i, step := range input.Steps {
		var deps []string
		for _, dep := range step.DependsOn {
			deps = append(deps, taskIDs[dep])
		}

		tools := step.Tools
		if len(tools) == 0 {
			tools = DefaultTaskTools
		}

		task := &tasks.Task{
			SessionID:   sessionID,
			Title:       step.Title,
			Description: step.Description,
			DependsOn:   deps,
			Tags:        step.ActorTags,
			Config: tasks.TaskConfig{
				Tools:                tools,
				WorkDir:              input.WorkDir,
				Env:                  input.Env,
				RequiredTags:         step.ActorTags,
				RequiredCapabilities: step.RequiredCapabilities,
			},
		}

		output, err := inliner.ExecuteInline(ctx, task)
		taskIDs[i] = task.ID

		entry := inlinePlanEntry{
			Step:   i,
			TaskID: task.ID,
			Title:  step.Title,
		}

		if err != nil {
			entry.Status = "failed"
			entry.Error = err.Error()
			overallStatus = "failed"
			results[i] = entry
			results = results[:i+1]
			break
		}

		if len(output) > 2000 {
			output = output[:2000] + "..."
		}
		entry.Status = "completed"
		entry.Output = output
		results[i] = entry
	}

	planID := "plan_" + strings.TrimPrefix(taskIDs[0], "task_")

	result, err := json.Marshal(inlinePlanResult{
		PlanID: planID,
		Title:  input.Title,
		Status: overallStatus,
		Tasks:  results,
	})
	if err != nil {
		return "", fmt.Errorf("submit_task: marshal plan result: %w", err)
	}
	return string(result), nil
}

func (t *SubmitTaskTool) runPlanAsync(input submitTaskInput, sessionID string) (string, error) {
	taskIDs := make([]string, len(input.Steps))
	entries := make([]planTaskEntry, len(input.Steps))

	for i, step := range input.Steps {
		var deps []string
		for _, dep := range step.DependsOn {
			deps = append(deps, taskIDs[dep])
		}

		tools := step.Tools
		if len(tools) == 0 {
			tools = DefaultTaskTools
		}

		task := &tasks.Task{
			SessionID:   sessionID,
			Title:       step.Title,
			Description: step.Description,
			DependsOn:   deps,
			Tags:        step.ActorTags,
			Config: tasks.TaskConfig{
				Tools:                tools,
				WorkDir:              input.WorkDir,
				Env:                  input.Env,
				RequiredTags:         step.ActorTags,
				RequiredCapabilities: step.RequiredCapabilities,
			},
		}

		if err := t.pool.Submit(task); err != nil {
			return "", fmt.Errorf("submit_task: submit step %d: %w", i, err)
		}

		taskIDs[i] = task.ID
		entries[i] = planTaskEntry{
			Step:   i,
			TaskID: task.ID,
			Title:  step.Title,
		}
	}

	planID := "plan_" + strings.TrimPrefix(taskIDs[0], "task_")

	result, err := json.Marshal(planTaskResult{
		PlanID: planID,
		Title:  input.Title,
		Tasks:  entries,
	})
	if err != nil {
		return "", fmt.Errorf("submit_task: marshal plan: %w", err)
	}
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
// query_tasks (unified check_task + list_tasks)
// =============================================================================

// QueryTasksTool retrieves task status (by ID) or lists tasks (by filter).
type QueryTasksTool struct {
	store tasks.Store
}

// NewQueryTasksTool creates a new query_tasks tool.
func NewQueryTasksTool(store tasks.Store) *QueryTasksTool {
	return &QueryTasksTool{store: store}
}

// QueryTasksManifest returns the plugin manifest for the query_tasks tool.
func QueryTasksManifest() *PluginManifest {
	return &PluginManifest{
		Name:        ToolQueryTasks,
		Description: "Query tasks: check a single task by ID or list tasks with filters",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        ToolQueryTasks,
				Description: "Query tasks. If task_id is provided, returns detailed status for that task. Otherwise lists tasks with optional filters (max 20, sorted by date).",
				Parameters: map[string]ParamSpec{
					"task_id": {
						Type:        "string",
						Description: "Single task ID to get detailed status (optional)",
					},
					"status": {
						Type:        "string",
						Description: "Filter by status: pending, running, completed, failed, cancelled",
						Enum:        []string{"pending", "running", "completed", "failed", "cancelled"},
					},
					"session_id": {
						Type:        "string",
						Description: "Filter by session ID",
					},
				},
			},
		},
	}
}

type queryTasksInput struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	SessionID string `json:"session_id"`
}

// queryTaskDetailOutput is the output for single-task detail mode.
type queryTaskDetailOutput struct {
	ID           string             `json:"id"`
	Title        string             `json:"title"`
	Status       tasks.TaskStatus   `json:"status"`
	Progress     tasks.TaskProgress `json:"progress"`
	ActorID      string             `json:"actor_id,omitempty"`
	ProviderName string             `json:"provider_name,omitempty"`
	Output       string             `json:"output,omitempty"`
	Error        string             `json:"error,omitempty"`
}

// queryTaskListEntry is an entry in the task list output.
type queryTaskListEntry struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Status    tasks.TaskStatus   `json:"status"`
	Progress  tasks.TaskProgress `json:"progress"`
	DependsOn []string           `json:"depends_on,omitempty"`
	CreatedAt string             `json:"created_at"`
}

// Info returns the tool info for Eino registration.
func (t *QueryTasksTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&QueryTasksManifest().Tools[0]), nil
}

// InvokableRun queries tasks: single task detail or filtered list.
func (t *QueryTasksTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input queryTasksInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("query_tasks: parse input: %w", err)
	}

	// Single task detail mode
	if input.TaskID != "" {
		task, err := t.store.Get(input.TaskID)
		if err != nil {
			return "", fmt.Errorf("query_tasks: %w", err)
		}

		out := queryTaskDetailOutput{
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
			return "", fmt.Errorf("query_tasks: marshal: %w", err)
		}
		return string(result), nil
	}

	// List mode
	filter := tasks.ListFilter{
		Status:    tasks.TaskStatus(input.Status),
		SessionID: input.SessionID,
	}

	all, err := t.store.List(filter)
	if err != nil {
		return "", fmt.Errorf("query_tasks: %w", err)
	}

	if len(all) > 20 {
		all = all[:20]
	}

	entries := make([]queryTaskListEntry, len(all))
	for i, task := range all {
		entries[i] = queryTaskListEntry{
			ID:        task.ID,
			Title:     task.Title,
			Status:    task.Status,
			Progress:  task.Progress,
			DependsOn: task.DependsOn,
			CreatedAt: task.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	result, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("query_tasks: marshal: %w", err)
	}
	return string(result), nil
}

var _ tool.InvokableTool = (*QueryTasksTool)(nil)

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
