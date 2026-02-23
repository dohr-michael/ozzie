package plugins

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/events"
)

// ToolActivator is the interface that activate_tools uses to activate tools.
// Implemented by agent.ToolSet (duck typing).
type ToolActivator interface {
	Activate(sessionID, toolName string) bool
	IsKnown(toolName string) bool
}

// ActivateToolsTool allows the agent to activate additional tools at runtime.
type ActivateToolsTool struct {
	activator ToolActivator
	registry  *ToolRegistry
}

// NewActivateToolsTool creates a new activate_tools tool.
func NewActivateToolsTool(activator ToolActivator, registry *ToolRegistry) *ActivateToolsTool {
	return &ActivateToolsTool{
		activator: activator,
		registry:  registry,
	}
}

// ActivateToolsManifest returns the plugin manifest for the activate_tools tool.
func ActivateToolsManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "activate_tools",
		Description: "Activate additional tools for the current session",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "activate_tools",
				Description: "Activate additional tools to make them available for use. Call this when you need a tool that is listed under 'Available Tools' but not yet active. The newly activated tools will be available on your next message.",
				Parameters: map[string]ParamSpec{
					"names": {
						Type:        "array",
						Description: "List of tool names to activate (e.g. [\"search\", \"git\"])",
						Required:    true,
					},
				},
			},
		},
	}
}

type activateToolsInput struct {
	Names []string `json:"names"`
}

type activateToolsOutput struct {
	Activated []activatedToolInfo `json:"activated"`
	Errors    []string            `json:"errors,omitempty"`
}

type activatedToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Info returns the tool info for Eino registration.
func (t *ActivateToolsTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ActivateToolsManifest().Tools[0]), nil
}

// InvokableRun activates the requested tools and returns their descriptions.
func (t *ActivateToolsTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	sessionID := events.SessionIDFromContext(ctx)
	if sessionID == "" {
		return "", fmt.Errorf("activate_tools: no session in context")
	}

	var input activateToolsInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("activate_tools: parse input: %w", err)
	}

	if len(input.Names) == 0 {
		return "", fmt.Errorf("activate_tools: names list is empty")
	}

	var out activateToolsOutput
	for _, name := range input.Names {
		if !t.activator.IsKnown(name) {
			out.Errors = append(out.Errors, fmt.Sprintf("unknown tool: %q", name))
			continue
		}
		if ok := t.activator.Activate(sessionID, name); !ok {
			out.Errors = append(out.Errors, fmt.Sprintf("failed to activate: %q", name))
			continue
		}
		desc := ""
		if spec := t.registry.ToolSpec(name); spec != nil {
			desc = spec.Description
		}
		out.Activated = append(out.Activated, activatedToolInfo{
			Name:        name,
			Description: desc,
		})
	}

	result, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("activate_tools: marshal result: %w", err)
	}
	return string(result), nil
}

var _ tool.InvokableTool = (*ActivateToolsTool)(nil)
