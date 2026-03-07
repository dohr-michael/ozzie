package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/events"
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

// --- activate_skill tool ---

// ActivateSkillTool loads a skill's instructions and activates its allowed tools.
type ActivateSkillTool struct {
	catalog   SkillCatalog
	activator ToolActivator
	registry  *ToolRegistry
}

// NewActivateSkillTool creates a new activate_skill tool.
func NewActivateSkillTool(catalog SkillCatalog, activator ToolActivator, registry *ToolRegistry) *ActivateSkillTool {
	return &ActivateSkillTool{
		catalog:   catalog,
		activator: activator,
		registry:  registry,
	}
}

// ActivateSkillManifest returns the plugin manifest for the activate_skill tool.
func ActivateSkillManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "activate_skill",
		Description: "Load a skill's full instructions and activate its tools",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "activate_skill",
				Description: "Load a skill's full instructions for progressive disclosure. Returns the skill's SKILL.md body, activates its allowed tools, and lists available resources (scripts, references, assets). For skills with a workflow, use run_workflow to execute the structured DAG.",
				Parameters: map[string]ParamSpec{
					"name": {
						Type:        "string",
						Description: "The name of the skill to activate",
						Required:    true,
					},
				},
			},
		},
	}
}

type activateSkillInput struct {
	Name string `json:"name"`
}

type activateSkillOutput struct {
	Body          string   `json:"body"`
	ActivatedTools []string `json:"activated_tools,omitempty"`
	Resources     []string `json:"resources,omitempty"`
	HasWorkflow   bool     `json:"has_workflow"`
}

// Info returns the tool info for Eino registration.
func (t *ActivateSkillTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ActivateSkillManifest().Tools[0]), nil
}

// InvokableRun loads the skill body and activates its tools.
func (t *ActivateSkillTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	sessionID := events.SessionIDFromContext(ctx)

	var input activateSkillInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("activate_skill: parse input: %w", err)
	}

	if input.Name == "" {
		return "", fmt.Errorf("activate_skill: name is required")
	}

	body, allowedTools, hasWorkflow, dir, err := t.catalog.SkillBody(input.Name)
	if err != nil {
		return "", fmt.Errorf("activate_skill: %w", err)
	}

	var out activateSkillOutput
	out.Body = body
	out.HasWorkflow = hasWorkflow

	// Activate allowed tools
	if sessionID != "" && len(allowedTools) > 0 {
		for _, toolName := range allowedTools {
			if t.activator.IsKnown(toolName) {
				if t.activator.Activate(sessionID, toolName) {
					out.ActivatedTools = append(out.ActivatedTools, toolName)
				}
			}
		}
	}

	// Auto-activate run_workflow if the skill has a workflow DAG
	if sessionID != "" && hasWorkflow {
		if t.activator.IsKnown("run_workflow") {
			if t.activator.Activate(sessionID, "run_workflow") {
				out.ActivatedTools = append(out.ActivatedTools, "run_workflow")
			}
		}
	}

	// List resources in scripts/, references/, assets/
	out.Resources = listResources(dir)

	result, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("activate_skill: marshal result: %w", err)
	}
	return string(result), nil
}

// listResources scans optional subdirectories for resources.
func listResources(dir string) []string {
	if dir == "" {
		return nil
	}

	var resources []string
	for _, subDir := range []string{"scripts", "references", "assets"} {
		fullPath := filepath.Join(dir, subDir)
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			resources = append(resources, filepath.Join(subDir, entry.Name()))
		}
	}
	return resources
}

var _ tool.InvokableTool = (*ActivateSkillTool)(nil)

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
