// Package mcp provides an MCP server that exposes Ozzie tools.
package mcp

import (
	"sort"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dohr-michael/ozzie/internal/plugins"
)

// toolSpecToMCPTool converts a plugins.ToolSpec to an mcp.Tool with JSON Schema.
func toolSpecToMCPTool(spec *plugins.ToolSpec) *mcpsdk.Tool {
	props := make(map[string]any, len(spec.Parameters))
	var required []string

	for name, p := range spec.Parameters {
		prop := map[string]any{
			"type":        p.Type,
			"description": p.Description,
		}
		if len(p.Enum) > 0 {
			prop["enum"] = p.Enum
		}
		if p.Default != nil {
			prop["default"] = p.Default
		}
		props[name] = prop

		if p.Required {
			required = append(required, name)
		}
	}

	// Sort required for deterministic output
	sort.Strings(required)

	inputSchema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		inputSchema["required"] = required
	}

	return &mcpsdk.Tool{
		Name:        spec.Name,
		Description: spec.Description,
		InputSchema: inputSchema,
	}
}
