package mcp

import (
	"context"
	"log/slog"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dohr-michael/ozzie/internal/plugins"
)

// NewMCPServer creates an MCP server exposing tools from the registry.
// If filter is non-empty, only tools matching the filter (by tool name or
// plugin name) are exposed.
func NewMCPServer(registry *plugins.ToolRegistry, filter string) *mcpsdk.Server {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "ozzie",
		Version: "0.1.0",
	}, nil)

	for _, name := range registry.ToolNames() {
		if filter != "" && !matchesFilter(registry, name, filter) {
			continue
		}

		spec := registry.ToolSpec(name)
		if spec == nil {
			continue
		}

		mcpTool := toolSpecToMCPTool(spec)

		// Capture tool in closure
		invokable := registry.Tool(name)
		toolName := name

		server.AddTool(mcpTool, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			args := string(req.Params.Arguments)
			result, err := invokable.InvokableRun(ctx, args)
			if err != nil {
				slog.Debug("mcp tool error", "tool", toolName, "error", err)
				return &mcpsdk.CallToolResult{
					IsError: true,
					Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: err.Error()}},
				}, nil
			}
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: result}},
			}, nil
		})

		slog.Debug("mcp tool registered", "tool", name)
	}

	return server
}

// matchesFilter checks if a tool name matches the filter.
// The filter can be a tool name or a plugin name (exposing all tools of that plugin).
func matchesFilter(registry *plugins.ToolRegistry, toolName, filter string) bool {
	if toolName == filter {
		return true
	}
	// Check if filter is a plugin name
	pluginTools := registry.PluginTools(filter)
	for _, pt := range pluginTools {
		if pt == toolName {
			return true
		}
	}
	return false
}
