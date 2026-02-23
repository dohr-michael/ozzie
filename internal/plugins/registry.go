package plugins

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"

	"github.com/dohr-michael/ozzie/internal/events"
)

// ToolRegistry is the unified registry for all tools (WASM + native).
type ToolRegistry struct {
	tools       map[string]tool.InvokableTool
	manifests   map[string]*PluginManifest // tool name → parent manifest
	specs       map[string]*ToolSpec       // tool name → specific ToolSpec
	pluginTools map[string][]string        // plugin name → tool names
	bus         *events.Bus
	runtime     *ExtismRuntime
}

// NewToolRegistry creates a new tool registry.
func NewToolRegistry(bus *events.Bus) *ToolRegistry {
	return &ToolRegistry{
		tools:       make(map[string]tool.InvokableTool),
		manifests:   make(map[string]*PluginManifest),
		specs:       make(map[string]*ToolSpec),
		pluginTools: make(map[string][]string),
		bus:         bus,
		runtime:     NewExtismRuntime(bus),
	}
}

// LoadWasmPlugin loads a single WASM plugin from its manifest file.
// A multi-tool plugin registers one entry per ToolSpec.
func (r *ToolRegistry) LoadWasmPlugin(ctx context.Context, manifestPath string) error {
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	// Resolve wasm_path relative to manifest directory
	if manifest.WasmPath != "" && !filepath.IsAbs(manifest.WasmPath) {
		manifest.WasmPath = filepath.Join(filepath.Dir(manifestPath), manifest.WasmPath)
	}

	wasmTools, err := r.runtime.Load(ctx, manifest)
	if err != nil {
		return err
	}

	var names []string
	for i, wt := range wasmTools {
		name := manifest.Tools[i].Name
		r.tools[name] = wt
		r.manifests[name] = manifest
		r.specs[name] = &manifest.Tools[i]
		names = append(names, name)
	}
	r.pluginTools[manifest.Name] = names
	return nil
}

// RegisterNative registers a Go-native tool with its manifest.
func (r *ToolRegistry) RegisterNative(name string, t tool.InvokableTool, manifest *PluginManifest) error {
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}
	r.tools[name] = t
	r.manifests[name] = manifest
	// Find matching ToolSpec by name
	for i := range manifest.Tools {
		if manifest.Tools[i].Name == name {
			r.specs[name] = &manifest.Tools[i]
			break
		}
	}
	// Track as plugin tools
	r.pluginTools[manifest.Name] = append(r.pluginTools[manifest.Name], name)
	return nil
}

// Tools returns all registered tools as a slice for the agent.
func (r *ToolRegistry) Tools() []tool.InvokableTool {
	result := make([]tool.InvokableTool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// Manifest returns the parent manifest for a given tool name.
func (r *ToolRegistry) Manifest(name string) *PluginManifest {
	return r.manifests[name]
}

// ToolSpec returns the specific ToolSpec for a given tool name.
func (r *ToolRegistry) ToolSpec(name string) *ToolSpec {
	return r.specs[name]
}

// PluginTools returns the tool names registered by a given plugin.
func (r *ToolRegistry) PluginTools(pluginName string) []string {
	return r.pluginTools[pluginName]
}

// ToolNames returns all registered tool names.
func (r *ToolRegistry) ToolNames() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// NativeToolNames returns the names of all tools whose manifest has Provider == "native".
func (r *ToolRegistry) NativeToolNames() []string {
	var names []string
	for name, m := range r.manifests {
		if m.Provider == "native" {
			names = append(names, name)
		}
	}
	return names
}

// Tool returns the InvokableTool for a given name, or nil if not found.
func (r *ToolRegistry) Tool(name string) tool.InvokableTool {
	return r.tools[name]
}

// ToolsByNames returns the InvokableTools matching the given names.
// Unknown names are silently skipped.
func (r *ToolRegistry) ToolsByNames(names []string) []tool.InvokableTool {
	result := make([]tool.InvokableTool, 0, len(names))
	for _, name := range names {
		if t, ok := r.tools[name]; ok {
			result = append(result, t)
		}
	}
	return result
}

// AllToolDescriptions returns a map of tool name → description for every
// registered tool. Tools without a ToolSpec get an empty description.
func (r *ToolRegistry) AllToolDescriptions() map[string]string {
	descs := make(map[string]string, len(r.tools))
	for name := range r.tools {
		if spec, ok := r.specs[name]; ok {
			descs[name] = spec.Description
		} else {
			descs[name] = ""
		}
	}
	return descs
}

// LoadPluginsDir scans a directory for plugin manifests and loads them.
// It looks for manifest.jsonc files in immediate subdirectories.
func (r *ToolRegistry) LoadPluginsDir(ctx context.Context, dir string, enabled []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("plugins directory not found, skipping", "dir", dir)
			return nil
		}
		return fmt.Errorf("read plugins dir: %w", err)
	}

	enabledSet := make(map[string]bool, len(enabled))
	for _, name := range enabled {
		enabledSet[name] = true
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(dir, entry.Name(), "manifest.jsonc")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			continue
		}

		// Check if enabled filter is active
		if len(enabledSet) > 0 && !enabledSet[entry.Name()] {
			slog.Debug("plugin skipped (not enabled)", "name", entry.Name())
			continue
		}

		if err := r.LoadWasmPlugin(ctx, manifestPath); err != nil {
			slog.Warn("failed to load plugin", "name", entry.Name(), "error", err)
			continue
		}
	}

	return nil
}

// Close releases all resources.
func (r *ToolRegistry) Close(ctx context.Context) {
	r.runtime.Close(ctx)
}
