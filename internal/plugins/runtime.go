package plugins

import (
	"context"
	"fmt"
	"log/slog"

	extism "github.com/extism/go-sdk"

	"github.com/dohr-michael/ozzie/internal/events"
)

// ExtismRuntime manages the lifecycle of WASM plugins.
type ExtismRuntime struct {
	bus     *events.Bus
	plugins map[string]*loadedPlugin
}

type loadedPlugin struct {
	manifest *PluginManifest
	plugin   *extism.Plugin
	kv       *KVStore
}

// NewExtismRuntime creates a new runtime for loading WASM plugins.
func NewExtismRuntime(bus *events.Bus) *ExtismRuntime {
	return &ExtismRuntime{
		bus:     bus,
		plugins: make(map[string]*loadedPlugin),
	}
}

// Load loads a WASM plugin from its manifest and returns one WasmTool per ToolSpec.
func (r *ExtismRuntime) Load(ctx context.Context, manifest *PluginManifest) ([]*WasmTool, error) {
	if manifest.Provider != "extism" {
		return nil, fmt.Errorf("runtime: unsupported provider %q", manifest.Provider)
	}
	if manifest.WasmPath == "" {
		return nil, fmt.Errorf("runtime: wasm_path is required for plugin %q", manifest.Name)
	}

	// Build Extism manifest with deny-by-default capabilities
	em := BuildExtismManifest(manifest)

	// Per-plugin KV store
	kv := NewKVStore()

	// Host functions
	hostFns := NewHostFunctions(r.bus, kv, manifest.Config)

	// Create plugin
	config := extism.PluginConfig{
		EnableWasi: true,
	}

	plugin, err := extism.NewPlugin(ctx, em, config, hostFns)
	if err != nil {
		return nil, fmt.Errorf("runtime: load plugin %q: %w", manifest.Name, err)
	}

	// Verify that each tool's Func export exists
	for _, ts := range manifest.Tools {
		if !plugin.FunctionExists(ts.Func) {
			plugin.Close(ctx)
			return nil, fmt.Errorf("runtime: plugin %q missing required %q export", manifest.Name, ts.Func)
		}
	}

	r.plugins[manifest.Name] = &loadedPlugin{
		manifest: manifest,
		plugin:   plugin,
		kv:       kv,
	}

	slog.Info("plugin loaded", "name", manifest.Name, "wasm", manifest.WasmPath, "tools", len(manifest.Tools))

	// Build one WasmTool per ToolSpec, all sharing the same plugin instance
	tools := make([]*WasmTool, len(manifest.Tools))
	for i := range manifest.Tools {
		tools[i] = &WasmTool{
			spec:       &manifest.Tools[i],
			plugin:     plugin,
			pluginName: manifest.Name,
		}
	}
	return tools, nil
}

// Close releases all loaded plugins.
func (r *ExtismRuntime) Close(ctx context.Context) {
	for name, lp := range r.plugins {
		if err := lp.plugin.Close(ctx); err != nil {
			slog.Warn("runtime: close plugin", "name", name, "error", err)
		}
	}
	r.plugins = nil
}
