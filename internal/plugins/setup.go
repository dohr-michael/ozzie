package plugins

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/events"
)

// SetupToolRegistry creates and populates a ToolRegistry with WASM and native tools.
// Tools are registered without dangerous wrappers — the caller is responsible for
// wrapping dangerous tools if needed (e.g. gateway wraps, MCP does not).
func SetupToolRegistry(ctx context.Context, cfg *config.Config, bus *events.Bus) (*ToolRegistry, error) {
	registry := NewToolRegistry(bus)

	pluginDir := cfg.Plugins.Dir
	if pluginDir == "" {
		pluginDir = filepath.Join(config.OzziePath(), "plugins")
	}
	if err := registry.LoadPluginsDir(ctx, pluginDir, cfg.Plugins.Enabled); err != nil {
		slog.Warn("failed to load plugins", "dir", pluginDir, "error", err)
	}

	// Register native tools (without dangerous wrapper)
	if err := registry.RegisterNative("cmd", NewCmdTool(0), CmdManifest()); err != nil {
		slog.Warn("failed to register cmd tool", "error", err)
	}
	if err := registry.RegisterNative("root_cmd", NewRootCmdTool(0), RootCmdManifest()); err != nil {
		slog.Warn("failed to register root_cmd tool", "error", err)
	}
	if err := registry.RegisterNative("read_file", NewReadFileTool(), ReadFileManifest()); err != nil {
		slog.Warn("failed to register read_file tool", "error", err)
	}
	if err := registry.RegisterNative("write_file", NewWriteFileTool(), WriteFileManifest()); err != nil {
		slog.Warn("failed to register write_file tool", "error", err)
	}
	if err := registry.RegisterNative("run_command", NewRunCmdTool(), RunCmdManifest()); err != nil {
		slog.Warn("failed to register run_command tool", "error", err)
	}
	if err := registry.RegisterNative("search", NewSearchTool(), SearchManifest()); err != nil {
		slog.Warn("failed to register search tool", "error", err)
	}
	if err := registry.RegisterNative("list_dir", NewListDirTool(), ListDirManifest()); err != nil {
		slog.Warn("failed to register list_dir tool", "error", err)
	}
	if err := registry.RegisterNative("git", NewGitTool(), GitManifest()); err != nil {
		slog.Warn("failed to register git tool", "error", err)
	}

	return registry, nil
}

// WrapRegistryDangerous wraps all dangerous tools in the registry with confirmation.
// Used by the gateway; MCP mode skips this.
func WrapRegistryDangerous(registry *ToolRegistry, bus *events.Bus) {
	for _, name := range registry.ToolNames() {
		spec := registry.ToolSpec(name)
		if spec != nil && spec.Dangerous {
			original := registry.tools[name]
			registry.tools[name] = WrapDangerous(original, name, true, bus)
		}
	}
}
