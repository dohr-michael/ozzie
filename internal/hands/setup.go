package hands

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/dohr-michael/ozzie/internal/agent"
	"github.com/dohr-michael/ozzie/internal/brain"
	"github.com/dohr-michael/ozzie/internal/brain/conscience"
	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/dohr-michael/ozzie/pkg/editor"
	editortools "github.com/dohr-michael/ozzie/pkg/editor/tools"
)

// SetupToolRegistry creates and populates a ToolRegistry with WASM and native tools.
// Tools are registered without dangerous wrappers — the caller is responsible for
// wrapping dangerous tools if needed (e.g. gateway wraps, MCP does not).
func SetupToolRegistry(ctx context.Context, cfg *config.Config, bus events.EventBus) (*ToolRegistry, error) {
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
func RegisterFilesystemTools(registry *ToolRegistry, backend *agent.OzzieBackend) {
	editorTool := editortools.NewStrReplaceEditorTool(editor.New(backend))
	if err := registry.RegisterNative("str_replace_editor", editorTool,
		resolvedNativeManifest(StrReplaceEditorManifest())); err != nil {
		slog.Warn("failed to register str_replace_editor tool", "error", err)
	}
}

// wrapToolDomain converts a registry tool to a domain tool, applies a wrapper,
// and converts back to Eino. Preserves the original Eino ToolInfo for schema generation.
func wrapToolDomain(registry *ToolRegistry, name string, wrapFn func(brain.Tool) brain.Tool) {
	original := registry.tools[name]
	einoInfo, _ := original.Info(context.Background())
	domain := agent.WrapEinoTool(original)
	wrapped := wrapFn(domain)
	registry.tools[name] = agent.UnwrapToEino(wrapped, einoInfo)
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
			wrapToolDomain(registry, name, func(t brain.Tool) brain.Tool {
				return conscience.WrapSandbox(t, name, conscience.SandboxExec, true, allowedPaths)
			})
		case resolved.Exec:
			wrapToolDomain(registry, name, func(t brain.Tool) brain.Tool {
				return conscience.WrapSandbox(t, name, conscience.SandboxExec, false, allowedPaths)
			})
		case resolved.Filesystem != nil && !resolved.Filesystem.ReadOnly:
			// Read-only filesystem tools (read_file, list_dir, search) are not sandboxed —
			// sub-agents may need to read reference files outside their WorkDir.
			wrapToolDomain(registry, name, func(t brain.Tool) brain.Tool {
				return conscience.WrapSandbox(t, name, conscience.SandboxFilesystem, false, allowedPaths)
			})
		}
	}
}

// WrapRegistryDangerous wraps all dangerous tools in the registry with confirmation.
// Used by the gateway; MCP mode skips this.
func WrapRegistryDangerous(registry *ToolRegistry, bus events.EventBus, perms *conscience.ToolPermissions) {
	for _, name := range registry.ToolNames() {
		spec := registry.ToolSpec(name)
		if spec != nil && spec.Dangerous {
			wrapToolDomain(registry, name, func(t brain.Tool) brain.Tool {
				return conscience.WrapDangerous(t, name, true, bus, perms)
			})
		}
	}
}

// WrapRegistryConstraints wraps all tools in the registry with constraint validation.
func WrapRegistryConstraints(registry *ToolRegistry) {
	for _, name := range registry.ToolNames() {
		wrapToolDomain(registry, name, func(t brain.Tool) brain.Tool {
			return conscience.WrapConstraint(t, name)
		})
	}
}
