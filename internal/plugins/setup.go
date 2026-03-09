package plugins

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/pkg/editor"
	editortools "github.com/dohr-michael/ozzie/pkg/editor/tools"
)

// SetupToolRegistry creates and populates a ToolRegistry with WASM and native tools.
// Tools are registered without dangerous wrappers — the caller is responsible for
// wrapping dangerous tools if needed (e.g. gateway wraps, MCP does not).
func SetupToolRegistry(ctx context.Context, cfg *config.Config, bus *events.Bus) (*ToolRegistry, error) {
	registry := NewToolRegistry(bus)

	// Convert config authorizations to plugin authorizations
	auths := make(map[string]*PluginAuthorization, len(cfg.Plugins.Authorizations))
	for name, authCfg := range cfg.Plugins.Authorizations {
		auths[name] = AuthFromConfig(authCfg)
	}

	pluginDir := cfg.Plugins.Dir
	if pluginDir == "" {
		pluginDir = filepath.Join(config.OzziePath(), "plugins")
	}
	if err := registry.LoadPluginsDir(ctx, pluginDir, cfg.Plugins.Enabled, auths); err != nil {
		slog.Warn("failed to load plugins", "dir", pluginDir, "error", err)
	}

	// Register native tools (without dangerous wrapper).
	// Native tools are auto-resolved: capabilities resolved with nil auth.
	// Filesystem tools (read_file, write_file, list_dir, search) are provided by the
	// Eino filesystem middleware and are NOT registered here.
	if err := registry.RegisterNative("run_command", NewExecuteTool(), resolvedNativeManifest(ExecuteManifest())); err != nil {
		slog.Warn("failed to register run_command tool", "error", err)
	}
	if err := registry.RegisterNative("git", NewGitTool(), resolvedNativeManifest(GitManifest())); err != nil {
		slog.Warn("failed to register git tool", "error", err)
	}

	// Register web tools (search + fetch)
	RegisterWebTools(ctx, cfg, registry)

	return registry, nil
}

// resolvedNativeManifest resolves capabilities for a native tool manifest.
// Native tools are auto-resolved with no external authorization.
func resolvedNativeManifest(m *PluginManifest) *PluginManifest {
	resolved := ResolveCapabilities(m.Capabilities, nil, m.ResourceLimits)
	m.Resolved = &resolved
	return m
}

// RegisterWebTools registers web_search and web_fetch native tools.
func RegisterWebTools(ctx context.Context, cfg *config.Config, registry *ToolRegistry) {
	if cfg.Web.Search.IsSearchEnabled() {
		searchTool, err := NewWebSearchTool(ctx, cfg.Web.Search)
		if err != nil {
			slog.Warn("failed to create web_search tool", "error", err)
		} else {
			if err := registry.RegisterNative("web_search", searchTool, resolvedNativeManifest(WebSearchManifest())); err != nil {
				slog.Warn("failed to register web_search tool", "error", err)
			}
		}
	}

	if cfg.Web.Fetch.IsFetchEnabled() {
		fetchTool := NewWebFetchTool(cfg.Web.Fetch)
		if err := registry.RegisterNative("web_fetch", fetchTool, resolvedNativeManifest(WebFetchManifest())); err != nil {
			slog.Warn("failed to register web_fetch tool", "error", err)
		}
	}
}

// RegisterFilesystemTools registers filesystem-based native tools (str_replace_editor).
// backend implements both filesystem.Backend (Eino) and editor.Backend.
func RegisterFilesystemTools(registry *ToolRegistry, backend *OzzieBackend) {
	editorTool := editortools.NewStrReplaceEditorTool(editor.New(backend))
	if err := registry.RegisterNative("str_replace_editor", editorTool,
		resolvedNativeManifest(StrReplaceEditorManifest())); err != nil {
		slog.Warn("failed to register str_replace_editor tool", "error", err)
	}
}

// WrapRegistrySandbox wraps exec and filesystem tools with sandbox validation.
// Must be called BEFORE WrapRegistryDangerous so the chain is:
// DangerousToolWrapper → SandboxGuard → inner tool.
func WrapRegistrySandbox(registry *ToolRegistry, allowedPaths []string) {
	for _, name := range registry.ToolNames() {
		manifest := registry.Manifest(name)
		if manifest == nil || manifest.Resolved == nil {
			continue
		}
		resolved := manifest.Resolved

		switch {
		case resolved.Elevated:
			// root_cmd — blocked unconditionally in autonomous mode
			registry.tools[name] = WrapSandbox(registry.tools[name], name, sandboxExec, true, allowedPaths)
		case resolved.Exec:
			registry.tools[name] = WrapSandbox(registry.tools[name], name, sandboxExec, false, allowedPaths)
		case resolved.Filesystem != nil && !resolved.Filesystem.ReadOnly:
			// Read-only filesystem tools (read_file, list_dir, search) are not sandboxed —
			// sub-agents may need to read reference files outside their WorkDir.
			registry.tools[name] = WrapSandbox(registry.tools[name], name, sandboxFilesystem, false, allowedPaths)
		}
	}
}

// WrapRegistryDangerous wraps all dangerous tools in the registry with confirmation.
// Used by the gateway; MCP mode skips this.
func WrapRegistryDangerous(registry *ToolRegistry, bus *events.Bus, perms *ToolPermissions) {
	for _, name := range registry.ToolNames() {
		spec := registry.ToolSpec(name)
		if spec != nil && spec.Dangerous {
			original := registry.tools[name]
			registry.tools[name] = WrapDangerous(original, name, true, bus, perms)
		}
	}
}
