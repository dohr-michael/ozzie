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

// PlanTaskTool creates a structured execution plan with dependent sub-tasks.
type PlanTaskTool struct {
	pool tasks.TaskSubmitter
}

// NewPlanTaskTool creates a new plan_task tool.
func NewPlanTaskTool(pool tasks.TaskSubmitter) *PlanTaskTool {
	return &PlanTaskTool{pool: pool}
}

// PlanTaskManifest returns the plugin manifest for the plan_task tool.
func PlanTaskManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "plan_task",
		Description: "Create a structured execution plan with dependent sub-tasks",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "plan_task",
				Description: "Create a structured execution plan with dependent sub-tasks. Each step becomes an async task. Use depends_on to declare dependencies between steps and control execution order.",
				Parameters: map[string]ParamSpec{
					"title": {
						Type:        "string",
						Description: "Title for the overall plan",
						Required:    true,
					},
					"work_dir": {
						Type:        "string",
						Description: "Absolute path to the working directory for all steps. Commands and file operations will use this as their base directory.",
					},
					"env": {
						Type:        "object",
						Description: "Additional environment variables for all steps. Example: {\"PROJECT_NAME\": \"chess\"}",
					},
					"steps": {
						Type:        "array",
						Description: "Ordered list of plan steps. Steps with no dependencies run in parallel. Use depends_on to enforce sequential execution where needed.",
						Required:    true,
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
									Description: "Indices (0-based) of steps that must complete before this step can start. Example: [0, 1] means this step depends on steps 0 and 1.",
									Items:       &ParamSpec{Type: "integer"},
								},
							},
						},
					},
				},
			},
		},
	}
}

type planTaskInput struct {
	Title   string            `json:"title"`
	WorkDir string            `json:"work_dir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Steps   []planStep        `json:"steps"`
}

type planStep struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tools       []string `json:"tools,omitempty"`
	DependsOn   []int    `json:"depends_on,omitempty"`
}

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

// Info returns the tool info for Eino registration.
func (t *PlanTaskTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&PlanTaskManifest().Tools[0]), nil
}

// InvokableRun creates a plan with dependent sub-tasks.
func (t *PlanTaskTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input planTaskInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("plan_task: parse input: %w", err)
	}
	if input.Title == "" {
		return "", fmt.Errorf("plan_task: title is required")
	}
	if len(input.Steps) == 0 {
		return "", fmt.Errorf("plan_task: at least one step is required")
	}

	// Validate depends_on indices
	for i, step := range input.Steps {
		for _, dep := range step.DependsOn {
			if dep < 0 || dep >= i {
				return "", fmt.Errorf("plan_task: step %d has invalid depends_on index %d (must be 0..%d)", i, dep, i-1)
			}
		}
	}

	sessionID := events.SessionIDFromContext(ctx)

	// Inline execution: when the pool has a single actor, execute steps
	// sequentially in the caller's goroutine to avoid deadlock.
	if inliner, ok := t.pool.(tasks.InlineExecutor); ok && inliner.ShouldInline() {
		return t.runInline(ctx, inliner, input, sessionID)
	}

	// Async path: submit all steps, let the scheduler handle execution.
	return t.runAsync(ctx, input, sessionID)
}

// inlinePlanResult is the JSON shape returned by inline plan execution.
type inlinePlanResult struct {
	PlanID string             `json:"plan_id"`
	Title  string             `json:"title"`
	Status string             `json:"status"` // "completed" or "failed"
	Tasks  []inlinePlanEntry  `json:"tasks"`
}

type inlinePlanEntry struct {
	Step   int    `json:"step"`
	TaskID string `json:"task_id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (t *PlanTaskTool) runInline(ctx context.Context, inliner tasks.InlineExecutor, input planTaskInput, sessionID string) (string, error) {
	taskIDs := make([]string, len(input.Steps))
	results := make([]inlinePlanEntry, len(input.Steps))
	overallStatus := "completed"

	for i, step := range input.Steps {
		// Convert step index dependencies to task ID dependencies
		var deps []string
		for _, dep := range step.DependsOn {
			deps = append(deps, taskIDs[dep])
		}

		tools := step.Tools
		if len(tools) == 0 {
			tools = []string{"run_command", "git", "query_memories"}
		}

		task := &tasks.Task{
			SessionID:   sessionID,
			Title:       step.Title,
			Description: step.Description,
			DependsOn:   deps,
			Config: tasks.TaskConfig{
				Tools:   tools,
				WorkDir: input.WorkDir,
				Env:     input.Env,
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
			// Short-circuit on failure: return partial results
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

	planID := "plan_" + taskIDs[0][5:] // strip "task_" prefix

	result, err := json.Marshal(inlinePlanResult{
		PlanID: planID,
		Title:  input.Title,
		Status: overallStatus,
		Tasks:  results,
	})
	if err != nil {
		return "", fmt.Errorf("plan_task: marshal inline result: %w", err)
	}
	return string(result), nil
}

func (t *PlanTaskTool) runAsync(_ context.Context, input planTaskInput, sessionID string) (string, error) {
	// Create tasks sequentially, mapping step indices to task IDs
	taskIDs := make([]string, len(input.Steps))
	entries := make([]planTaskEntry, len(input.Steps))

	for i, step := range input.Steps {
		// Convert step index dependencies to task ID dependencies
		var deps []string
		for _, dep := range step.DependsOn {
			deps = append(deps, taskIDs[dep])
		}

		// Default tools if step doesn't specify any
		tools := step.Tools
		if len(tools) == 0 {
			tools = []string{"run_command", "git", "query_memories"}
		}

		task := &tasks.Task{
			SessionID:   sessionID,
			Title:       step.Title,
			Description: step.Description,
			DependsOn:   deps,
			Config: tasks.TaskConfig{
				Tools:   tools,
				WorkDir: input.WorkDir,
				Env:     input.Env,
			},
		}

		if err := t.pool.Submit(task); err != nil {
			return "", fmt.Errorf("plan_task: submit step %d: %w", i, err)
		}

		taskIDs[i] = task.ID
		entries[i] = planTaskEntry{
			Step:   i,
			TaskID: task.ID,
			Title:  step.Title,
		}
	}

	// Use first task ID as plan ID prefix
	planID := "plan_" + taskIDs[0][5:] // strip "task_" prefix

	result, err := json.Marshal(planTaskResult{
		PlanID: planID,
		Title:  input.Title,
		Tasks:  entries,
	})
	if err != nil {
		return "", fmt.Errorf("plan_task: marshal: %w", err)
	}
	return string(result), nil
}

var _ tool.InvokableTool = (*PlanTaskTool)(nil)
