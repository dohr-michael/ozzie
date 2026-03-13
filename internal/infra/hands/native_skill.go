package hands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// SkillCatalog provides read-only access to skills for the native tools.
// Implemented by skills.Registry (avoids import cycle plugins → skills).
type SkillCatalog interface {
	SkillBody(name string) (body string, allowedTools []string, hasWorkflow bool, dir string, err error)
	Names() []string
}

// WorkflowExecutor runs a skill's workflow DAG.
// Implemented by skills.PoolSkillExecutor (avoids import cycle plugins → skills).
type WorkflowExecutor interface {
	RunWorkflow(ctx context.Context, skillName string, vars map[string]string) (string, error)
}

// --- run_workflow tool ---

// RunWorkflowTool executes a skill's structured workflow DAG.
type RunWorkflowTool struct {
	executor WorkflowExecutor
}

// NewRunWorkflowTool creates a new run_workflow tool.
func NewRunWorkflowTool(executor WorkflowExecutor) *RunWorkflowTool {
	return &RunWorkflowTool{executor: executor}
}

// RunWorkflowManifest returns the plugin manifest for the run_workflow tool.
func RunWorkflowManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "run_workflow",
		Description: "Execute a skill's structured workflow DAG",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "run_workflow",
				Description: "Execute the structured workflow (DAG) defined in a skill's workflow.yaml. Each step runs an ephemeral agent with its own instruction, tools, and optional acceptance criteria. Steps run in parallel where dependencies allow.",
				Parameters: map[string]ParamSpec{
					"skill_name": {
						Type:        "string",
						Description: "The name of the skill whose workflow to execute",
						Required:    true,
					},
					"vars": {
						Type:        "object",
						Description: "Variables to pass to the workflow (key-value pairs)",
					},
				},
			},
		},
	}
}

type runWorkflowInput struct {
	SkillName string            `json:"skill_name"`
	Vars      map[string]string `json:"vars"`
}

// Info returns the tool info for Eino registration.
func (t *RunWorkflowTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&RunWorkflowManifest().Tools[0]), nil
}

// InvokableRun executes the workflow.
func (t *RunWorkflowTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input runWorkflowInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("run_workflow: parse input: %w", err)
	}

	if input.SkillName == "" {
		return "", fmt.Errorf("run_workflow: skill_name is required")
	}

	vars := input.Vars
	if vars == nil {
		vars = make(map[string]string)
	}

	output, err := t.executor.RunWorkflow(ctx, input.SkillName, vars)
	if err != nil {
		return "", fmt.Errorf("run_workflow: %w", err)
	}

	// Return structured result
	result := map[string]string{
		"skill":  input.SkillName,
		"output": output,
	}
	if len(vars) > 0 {
		result["vars"] = strings.Join(mapToSlice(vars), ", ")
	}
	out, _ := json.Marshal(result)
	return string(out), nil
}

func mapToSlice(m map[string]string) []string {
	result := make([]string, 0, len(m))
	for k, v := range m {
		result = append(result, k+"="+v)
	}
	return result
}

var _ tool.InvokableTool = (*RunWorkflowTool)(nil)
