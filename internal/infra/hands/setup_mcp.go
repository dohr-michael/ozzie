package hands

import (
	"context"
	"log/slog"
	"time"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SetupMCPServers connects to external MCP servers, discovers their tools,
// and registers them in the ToolRegistry.
func SetupMCPServers(ctx context.Context, cfg config.MCPConfig, registry *ToolRegistry, bus events.EventBus) error {
	if len(cfg.Servers) == 0 {
		return nil
	}

	manager := NewMCPManager()
	registry.mcpManager = manager

	for name, serverCfg := range cfg.Servers {
		if err := setupOneMCPServer(ctx, name, serverCfg, manager, registry, bus); err != nil {
			slog.Warn("mcp server setup failed", "server", name, "error", err)
			continue
		}
	}

	return nil
}

func setupOneMCPServer(ctx context.Context, name string, cfg *config.MCPServerConfig, manager *MCPManager, registry *ToolRegistry, bus events.EventBus) error {
	session, err := manager.Connect(ctx, name, cfg)
	if err != nil {
		return err
	}

	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		return err
	}

	deniedSet := make(map[string]bool, len(cfg.DeniedTools))
	for _, t := range cfg.DeniedTools {
		deniedSet[t] = true
	}
	allowedSet := make(map[string]bool, len(cfg.AllowedTools))
	for _, t := range cfg.AllowedTools {
		allowedSet[t] = true
	}
	trustedSet := make(map[string]bool, len(cfg.TrustedTools))
	for _, t := range cfg.TrustedTools {
		trustedSet[t] = true
	}

	timeout := time.Duration(cfg.Timeout) * time.Millisecond
	registered := 0

	for _, mcpTool := range toolsResult.Tools {
		if !isToolAllowed(mcpTool.Name, allowedSet, deniedSet) {
			slog.Debug("mcp tool filtered", "server", name, "tool", mcpTool.Name)
			continue
		}

		prefixedName := name + "__" + mcpTool.Name
		proxyTool := &MCPTool{
			serverName: name,
			toolName:   mcpTool.Name,
			session:    session,
			mcpTool:    mcpTool,
			timeout:    timeout,
		}

		isDangerous := cfg.IsDangerous() && !trustedSet[mcpTool.Name]
		manifest := mcpToolManifest(name, mcpTool, isDangerous)
		if err := registry.RegisterNative(prefixedName, proxyTool, resolvedNativeManifest(manifest)); err != nil {
			slog.Warn("mcp tool register failed", "server", name, "tool", mcpTool.Name, "error", err)
			continue
		}
		registered++
	}

	slog.Info("mcp server connected", "server", name, "tools", registered)

	bus.Publish(events.NewEvent(events.EventToolCall, events.SourceMCP, map[string]any{
		"action": "connected",
		"server": name,
		"tools":  registered,
	}))

	return nil
}

// isToolAllowed checks if a tool passes the allowed/denied filters.
func isToolAllowed(toolName string, allowed, denied map[string]bool) bool {
	if denied[toolName] {
		return false
	}
	if len(allowed) > 0 {
		return allowed[toolName]
	}
	return true
}

// mcpToolManifest builds a synthetic PluginManifest for an MCP tool.
func mcpToolManifest(serverName string, t *mcp.Tool, dangerous bool) *PluginManifest {
	spec := mcpToolToToolSpec(serverName, t)
	spec.Dangerous = dangerous

	return &PluginManifest{
		Name:        serverName + "__" + t.Name,
		Description: t.Description,
		Level:       "tool",
		Provider:    "mcp",
		Dangerous:   dangerous,
		Tools:       []ToolSpec{spec},
	}
}
