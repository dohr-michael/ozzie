package hands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/core/events"
)

// ToolActivator is the interface that activate uses to activate tools.
// Implemented by brain.ToolSet (duck typing).
type ToolActivator interface {
	Activate(sessionID, toolName string) bool
	IsKnown(toolName string) bool
}

// =============================================================================
// activate (unified tool + skill activation)
// =============================================================================

// ActivateTool allows the agent to activate tools or skills at runtime.
type ActivateTool struct {
	activator ToolActivator
	registry  *ToolRegistry
	catalog   SkillCatalog // optional — nil if no skills
}

// NewActivateTool creates a new activate tool.
func NewActivateTool(activator ToolActivator, registry *ToolRegistry, catalog SkillCatalog) *ActivateTool {
	return &ActivateTool{
		activator: activator,
		registry:  registry,
		catalog:   catalog,
	}
}

// ActivateManifest returns the plugin manifest for the activate tool.
func ActivateManifest() *PluginManifest {
	return &PluginManifest{
		Name:        ToolActivate,
		Description: "Activate tools or skills for the current session",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        ToolActivate,
				Description: "Activate additional tools or skills to make them available. For tools: activates them for use on the next message. For skills: loads the skill's instructions and activates its allowed tools. Skills with a workflow will also activate run_workflow.",
				Parameters: map[string]ParamSpec{
					"names": {
						Type:        "array",
						Description: "List of tool or skill names to activate (e.g. [\"docker_build\", \"deploy\"])",
						Required:    true,
					},
				},
			},
		},
	}
}

type activateInput struct {
	Names []string `json:"names"`
}

type activateOutput struct {
	Activated []activatedEntry `json:"activated"`
	Errors    []string         `json:"errors,omitempty"`
}

type activatedEntry struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "tool" or "skill"
	Description string   `json:"description,omitempty"`
	Body        string   `json:"body,omitempty"`         // skill body (instructions)
	Tools       []string `json:"tools,omitempty"`         // activated tools (skill)
	Resources   []string `json:"resources,omitempty"`     // skill resources
	HasWorkflow bool     `json:"has_workflow,omitempty"`
}

// Info returns the tool info for Eino registration.
func (t *ActivateTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ActivateManifest().Tools[0]), nil
}

// InvokableRun activates tools or skills by name.
func (t *ActivateTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	sessionID := events.SessionIDFromContext(ctx)
	if sessionID == "" {
		return "", fmt.Errorf("activate: no session in context")
	}

	var input activateInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("activate: parse input: %w", err)
	}

	if len(input.Names) == 0 {
		return "", fmt.Errorf("activate: names list is empty")
	}

	var out activateOutput
	for _, name := range input.Names {
		// Try tool first
		if t.activator.IsKnown(name) {
			if ok := t.activator.Activate(sessionID, name); !ok {
				out.Errors = append(out.Errors, fmt.Sprintf("failed to activate tool: %q", name))
				continue
			}
			desc := ""
			if spec := t.registry.ToolSpec(name); spec != nil {
				desc = spec.Description
			}
			out.Activated = append(out.Activated, activatedEntry{
				Name:        name,
				Type:        "tool",
				Description: desc,
			})
			continue
		}

		// Try skill
		if t.catalog != nil {
			body, allowedTools, hasWorkflow, dir, err := t.catalog.SkillBody(name)
			if err == nil {
				entry := activatedEntry{
					Name:        name,
					Type:        "skill",
					Body:        body,
					HasWorkflow: hasWorkflow,
					Resources:   listResources(dir),
				}

				// Activate allowed tools
				for _, toolName := range allowedTools {
					if t.activator.IsKnown(toolName) {
						if t.activator.Activate(sessionID, toolName) {
							entry.Tools = append(entry.Tools, toolName)
						}
					}
				}

				// Auto-activate run_workflow if skill has a workflow
				if hasWorkflow && t.activator.IsKnown("run_workflow") {
					if t.activator.Activate(sessionID, "run_workflow") {
						entry.Tools = append(entry.Tools, "run_workflow")
					}
				}

				out.Activated = append(out.Activated, entry)
				continue
			}
		}

		out.Errors = append(out.Errors, fmt.Sprintf("unknown tool or skill: %q", name))
	}

	result, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("activate: marshal result: %w", err)
	}
	return string(result), nil
}

var _ tool.InvokableTool = (*ActivateTool)(nil)

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

