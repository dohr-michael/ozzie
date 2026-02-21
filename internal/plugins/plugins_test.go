package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dohr-michael/ozzie/internal/events"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.jsonc")

	content := `{
		// Test manifest
		"name": "test_plugin",
		"description": "A test plugin",
		"level": "tool",
		"provider": "extism",
		"wasm_path": "test.wasm",
		"dangerous": false,
		"capabilities": {
			"kv": true,
			"log": true
		},
		"tools": [
			{
				"name": "test_tool",
				"description": "A test tool",
				"parameters": {
					"input": {
						"type": "string",
						"description": "Test input",
						"required": true
					}
				}
			}
		]
	}`

	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if m.Name != "test_plugin" {
		t.Errorf("Name = %q, want %q", m.Name, "test_plugin")
	}
	if m.Provider != "extism" {
		t.Errorf("Provider = %q, want %q", m.Provider, "extism")
	}
	if !m.Capabilities.KV {
		t.Error("Capabilities.KV = false, want true")
	}
	if !m.Capabilities.Log {
		t.Error("Capabilities.Log = false, want true")
	}
	if len(m.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(m.Tools))
	}
	if m.Tools[0].Name != "test_tool" {
		t.Errorf("Tools[0].Name = %q, want %q", m.Tools[0].Name, "test_tool")
	}
	if len(m.Tools[0].Parameters) != 1 {
		t.Errorf("Tools[0].Parameters len = %d, want 1", len(m.Tools[0].Parameters))
	}
	p, ok := m.Tools[0].Parameters["input"]
	if !ok {
		t.Fatal("Tools[0].Parameters missing 'input'")
	}
	if p.Type != "string" {
		t.Errorf("Parameter type = %q, want %q", p.Type, "string")
	}
	if !p.Required {
		t.Error("Parameter required = false, want true")
	}
	// Default Func
	if m.Tools[0].Func != "handle" {
		t.Errorf("Tools[0].Func = %q, want %q", m.Tools[0].Func, "handle")
	}
}

func TestLoadManifest_MissingName(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.jsonc")

	content := `{"description": "no name", "tools": [{"name": "t", "description": "d"}]}`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(manifestPath)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestLoadManifest_MissingTools(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.jsonc")

	content := `{"name": "test"}`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(manifestPath)
	if err == nil {
		t.Error("expected error for missing tools")
	}
}

func TestLoadManifest_DefaultToolName(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.jsonc")

	content := `{
		"name": "my_plugin",
		"tools": [
			{
				"description": "A tool"
			}
		]
	}`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Tools[0].Name != "my_plugin" {
		t.Errorf("Tools[0].Name = %q, want %q (should default to manifest name)", m.Tools[0].Name, "my_plugin")
	}
}

func TestLoadManifest_MultiTool(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.jsonc")

	content := `{
		"name": "multi",
		"description": "Multi-tool plugin",
		"provider": "extism",
		"wasm_path": "multi.wasm",
		"dangerous": true,
		"tools": [
			{
				"name": "list_items",
				"description": "List all items",
				"func": "list_items",
				"parameters": {}
			},
			{
				"name": "create_item",
				"description": "Create an item",
				"func": "create_item",
				"dangerous": true,
				"parameters": {
					"name": {"type": "string", "required": true}
				}
			}
		]
	}`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if len(m.Tools) != 2 {
		t.Fatalf("Tools len = %d, want 2", len(m.Tools))
	}

	// First tool
	if m.Tools[0].Name != "list_items" {
		t.Errorf("Tools[0].Name = %q, want %q", m.Tools[0].Name, "list_items")
	}
	if m.Tools[0].Func != "list_items" {
		t.Errorf("Tools[0].Func = %q, want %q", m.Tools[0].Func, "list_items")
	}
	// Dangerous propagated from manifest level
	if !m.Tools[0].Dangerous {
		t.Error("Tools[0].Dangerous = false, want true (propagated from manifest)")
	}

	// Second tool
	if m.Tools[1].Name != "create_item" {
		t.Errorf("Tools[1].Name = %q, want %q", m.Tools[1].Name, "create_item")
	}
	if m.Tools[1].Func != "create_item" {
		t.Errorf("Tools[1].Func = %q, want %q", m.Tools[1].Func, "create_item")
	}
	if !m.Tools[1].Dangerous {
		t.Error("Tools[1].Dangerous = false, want true")
	}
}

func TestLoadManifest_MultiToolRequiresName(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.jsonc")

	// Multi-tool without names should fail
	content := `{
		"name": "multi",
		"tools": [
			{"description": "first"},
			{"description": "second"}
		]
	}`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadManifest(manifestPath)
	if err == nil {
		t.Error("expected error for multi-tool without names")
	}
}

func TestBuildExtismManifest_DenyByDefault(t *testing.T) {
	m := &PluginManifest{
		Name:     "test",
		WasmPath: "/tmp/test.wasm",
		Tools:    []ToolSpec{{Name: "test", Func: "handle"}},
	}

	em := BuildExtismManifest(m)

	if len(em.AllowedHosts) != 0 {
		t.Errorf("AllowedHosts = %v, want empty (deny-by-default)", em.AllowedHosts)
	}
	if len(em.AllowedPaths) != 0 {
		t.Errorf("AllowedPaths = %v, want empty (deny-by-default)", em.AllowedPaths)
	}
}

func TestBuildExtismManifest_WithCapabilities(t *testing.T) {
	m := &PluginManifest{
		Name:     "test",
		WasmPath: "/tmp/test.wasm",
		Tools:    []ToolSpec{{Name: "test", Func: "handle"}},
		Capabilities: CapabilitySet{
			HTTP: &HTTPCapability{
				AllowedHosts: []string{"api.example.com"},
			},
			Filesystem: &FSCapability{
				AllowedPaths: map[string]string{"/data": "/mnt"},
			},
			Memory: &MemoryLimit{
				MaxPages: 16,
			},
			Timeout: 5000,
		},
	}

	em := BuildExtismManifest(m)

	if len(em.AllowedHosts) != 1 || em.AllowedHosts[0] != "api.example.com" {
		t.Errorf("AllowedHosts = %v, want [api.example.com]", em.AllowedHosts)
	}
	if em.AllowedPaths["/data"] != "/mnt" {
		t.Errorf("AllowedPaths = %v, want {\"/data\": \"/mnt\"}", em.AllowedPaths)
	}
	if em.Memory == nil || em.Memory.MaxPages != 16 {
		t.Errorf("Memory.MaxPages = %v, want 16", em.Memory)
	}
	if em.Timeout != 5000 {
		t.Errorf("Timeout = %d, want 5000", em.Timeout)
	}
}

func TestKVStore(t *testing.T) {
	kv := NewKVStore()

	// Get non-existent key
	if v := kv.Get("missing"); v != nil {
		t.Errorf("Get(missing) = %v, want nil", v)
	}

	// Set and get
	kv.Set("key1", []byte("value1"))
	if v := kv.Get("key1"); string(v) != "value1" {
		t.Errorf("Get(key1) = %q, want %q", string(v), "value1")
	}

	// Overwrite
	kv.Set("key1", []byte("value2"))
	if v := kv.Get("key1"); string(v) != "value2" {
		t.Errorf("Get(key1) after overwrite = %q, want %q", string(v), "value2")
	}
}

func TestToolSpecToToolInfo(t *testing.T) {
	spec := &ToolSpec{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]ParamSpec{
			"name": {
				Type:        "string",
				Description: "The name",
				Required:    true,
			},
			"count": {
				Type:        "integer",
				Description: "A count",
				Required:    false,
			},
		},
	}

	info := toolSpecToToolInfo(spec)

	if info.Name != "test_tool" {
		t.Errorf("Name = %q, want %q", info.Name, "test_tool")
	}
	if info.Desc != "A test tool" {
		t.Errorf("Desc = %q, want %q", info.Desc, "A test tool")
	}
	if info.ParamsOneOf == nil {
		t.Fatal("ParamsOneOf is nil")
	}
}

func TestToolRegistry_RegisterNative(t *testing.T) {
	bus := events.NewBus(16)
	defer bus.Close()

	registry := NewToolRegistry(bus)
	defer registry.Close(context.Background())

	cmdTool := NewCmdTool(0)
	err := registry.RegisterNative("cmd", cmdTool, CmdManifest())
	if err != nil {
		t.Fatalf("RegisterNative: %v", err)
	}

	tools := registry.Tools()
	if len(tools) != 1 {
		t.Fatalf("Tools() len = %d, want 1", len(tools))
	}

	// ToolSpec should be retrievable
	spec := registry.ToolSpec("cmd")
	if spec == nil {
		t.Fatal("ToolSpec(cmd) = nil")
	}
	if spec.Name != "cmd" {
		t.Errorf("ToolSpec.Name = %q, want %q", spec.Name, "cmd")
	}

	// PluginTools should return the tool name
	pt := registry.PluginTools("cmd")
	if len(pt) != 1 || pt[0] != "cmd" {
		t.Errorf("PluginTools(cmd) = %v, want [cmd]", pt)
	}

	// Duplicate registration should fail
	err = registry.RegisterNative("cmd", cmdTool, CmdManifest())
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestToolRegistry_LoadPluginsDir_NoDir(t *testing.T) {
	bus := events.NewBus(16)
	defer bus.Close()

	registry := NewToolRegistry(bus)
	defer registry.Close(context.Background())

	// Non-existent directory should not error
	err := registry.LoadPluginsDir(context.Background(), "/nonexistent/path", nil)
	if err != nil {
		t.Fatalf("LoadPluginsDir non-existent: %v", err)
	}
}

func TestCmdTool_Info(t *testing.T) {
	cmdTool := NewCmdTool(0)
	info, err := cmdTool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "cmd" {
		t.Errorf("Name = %q, want %q", info.Name, "cmd")
	}
}

func TestRootCmdTool_Info(t *testing.T) {
	tool := NewRootCmdTool(0)
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "root_cmd" {
		t.Errorf("Name = %q, want %q", info.Name, "root_cmd")
	}
}

func TestCmdTool_InvokableRun(t *testing.T) {
	cmdTool := NewCmdTool(0)
	result, err := cmdTool.InvokableRun(context.Background(), `{"command": "echo hello"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	// Should contain "hello" in stdout
	if !contains(result, "hello") {
		t.Errorf("result %q does not contain 'hello'", result)
	}
}

func TestCmdTool_InvokableRun_EmptyCommand(t *testing.T) {
	cmdTool := NewCmdTool(0)
	_, err := cmdTool.InvokableRun(context.Background(), `{"command": ""}`)
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestWasmToolIntegration(t *testing.T) {
	// This test requires WASM plugins built with TinyGo (which exports functions properly).
	wasmPath := filepath.Join("..", "..", "plugins", "calculator", "calculator.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("calculator.wasm not built, run 'make build-plugins' first")
	}

	bus := events.NewBus(16)
	defer bus.Close()

	manifest := &PluginManifest{
		Name:     "calculator",
		Provider: "extism",
		WasmPath: wasmPath,
		Tools: []ToolSpec{
			{
				Name:        "calculator",
				Description: "Evaluate math expressions",
				Func:        "handle",
				Parameters: map[string]ParamSpec{
					"expression": {Type: "string", Required: true},
				},
			},
		},
	}

	runtime := NewExtismRuntime(bus)
	defer runtime.Close(context.Background())

	wasmTools, err := runtime.Load(context.Background(), manifest)
	if err != nil {
		// Standard Go wasip1 builds don't export functions for Extism — skip gracefully
		t.Skipf("could not load WASM plugin (may need TinyGo build): %v", err)
	}

	if len(wasmTools) != 1 {
		t.Fatalf("Load returned %d tools, want 1", len(wasmTools))
	}

	result, err := wasmTools[0].InvokableRun(context.Background(), `{"expression": "2 + 2"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	if !contains(result, "4") {
		t.Errorf("result %q does not contain '4'", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
