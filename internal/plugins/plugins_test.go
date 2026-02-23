package plugins

import (
	"context"
	"os"
	"os/exec"
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

// --- ReadFileTool tests ---

func TestReadFileTool_Info(t *testing.T) {
	tool := NewReadFileTool()
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "read_file" {
		t.Errorf("Name = %q, want %q", info.Name, "read_file")
	}
	if info.ParamsOneOf == nil {
		t.Error("ParamsOneOf is nil")
	}
}

func TestReadFileTool_InvokableRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewReadFileTool()
	result, err := tool.InvokableRun(context.Background(), `{"path": "`+path+`"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, "line1") {
		t.Errorf("result %q does not contain 'line1'", result)
	}
	if !contains(result, `"lines":4`) {
		t.Errorf("result %q does not contain lines count", result)
	}
}

func TestReadFileTool_InvokableRun_OffsetLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\nd\ne\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewReadFileTool()
	result, err := tool.InvokableRun(context.Background(), `{"path": "`+path+`", "offset": 1, "limit": 2}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, `"truncated":true`) {
		t.Errorf("expected truncated=true in result %q", result)
	}
	if !contains(result, "b") {
		t.Errorf("result %q does not contain 'b'", result)
	}
}

func TestReadFileTool_InvokableRun_FileNotFound(t *testing.T) {
	tool := NewReadFileTool()
	_, err := tool.InvokableRun(context.Background(), `{"path": "/nonexistent/file.txt"}`)
	if err == nil {
		t.Error("expected error for file not found")
	}
}

func TestReadFileTool_InvokableRun_EmptyPath(t *testing.T) {
	tool := NewReadFileTool()
	_, err := tool.InvokableRun(context.Background(), `{"path": ""}`)
	if err == nil {
		t.Error("expected error for empty path")
	}
}

// --- WriteFileTool tests ---

func TestWriteFileTool_Info(t *testing.T) {
	tool := NewWriteFileTool()
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "write_file" {
		t.Errorf("Name = %q, want %q", info.Name, "write_file")
	}
}

func TestWriteFileTool_InvokableRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")

	tool := NewWriteFileTool()
	result, err := tool.InvokableRun(context.Background(), `{"path": "`+path+`", "content": "hello world"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, `"bytes_written":11`) {
		t.Errorf("result %q does not contain bytes_written", result)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("file content = %q, want %q", string(data), "hello world")
	}
}

func TestWriteFileTool_InvokableRun_CreateDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "file.txt")

	tool := NewWriteFileTool()
	_, err := tool.InvokableRun(context.Background(), `{"path": "`+path+`", "content": "nested"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("file content = %q, want %q", string(data), "nested")
	}
}

func TestWriteFileTool_InvokableRun_EmptyPath(t *testing.T) {
	tool := NewWriteFileTool()
	_, err := tool.InvokableRun(context.Background(), `{"path": "", "content": "x"}`)
	if err == nil {
		t.Error("expected error for empty path")
	}
}

// --- RunCmdTool tests ---

func TestRunCmdTool_Info(t *testing.T) {
	tool := NewRunCmdTool()
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "run_command" {
		t.Errorf("Name = %q, want %q", info.Name, "run_command")
	}
}

func TestRunCmdTool_InvokableRun(t *testing.T) {
	tool := NewRunCmdTool()
	result, err := tool.InvokableRun(context.Background(), `{"command": "echo hello"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, "hello") {
		t.Errorf("result %q does not contain 'hello'", result)
	}
}

func TestRunCmdTool_InvokableRun_Timeout(t *testing.T) {
	tool := NewRunCmdTool()
	_, err := tool.InvokableRun(context.Background(), `{"command": "sleep 10", "timeout": 1}`)
	if err == nil {
		t.Error("expected error for timeout")
	}
}

func TestRunCmdTool_InvokableRun_EmptyCommand(t *testing.T) {
	tool := NewRunCmdTool()
	_, err := tool.InvokableRun(context.Background(), `{"command": ""}`)
	if err == nil {
		t.Error("expected error for empty command")
	}
}

// --- SearchTool tests ---

func TestSearchTool_Info(t *testing.T) {
	tool := NewSearchTool()
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "search" {
		t.Errorf("Name = %q, want %q", info.Name, "search")
	}
}

func TestSearchTool_InvokableRun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world\nfoo bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\nfunc hello() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewSearchTool()
	result, err := tool.InvokableRun(context.Background(), `{"pattern": "hello", "path": "`+dir+`"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, "hello") {
		t.Errorf("result %q does not contain 'hello'", result)
	}
	if !contains(result, `"total":2`) {
		t.Errorf("result %q does not contain total:2", result)
	}
}

func TestSearchTool_InvokableRun_InvalidRegex(t *testing.T) {
	tool := NewSearchTool()
	_, err := tool.InvokableRun(context.Background(), `{"pattern": "[invalid"}`)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestSearchTool_InvokableRun_GlobFilter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("match here\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("match here too\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewSearchTool()
	result, err := tool.InvokableRun(context.Background(), `{"pattern": "match", "path": "`+dir+`", "glob": "*.txt"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, `"total":1`) {
		t.Errorf("expected total:1, got %q", result)
	}
}

func TestSearchTool_InvokableRun_EmptyPattern(t *testing.T) {
	tool := NewSearchTool()
	_, err := tool.InvokableRun(context.Background(), `{"pattern": ""}`)
	if err == nil {
		t.Error("expected error for empty pattern")
	}
}

// --- ListDirTool tests ---

func TestListDirTool_Info(t *testing.T) {
	tool := NewListDirTool()
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "list_dir" {
		t.Errorf("Name = %q, want %q", info.Name, "list_dir")
	}
}

func TestListDirTool_InvokableRun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	tool := NewListDirTool()
	result, err := tool.InvokableRun(context.Background(), `{"path": "`+dir+`"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, "a.txt") {
		t.Errorf("result %q does not contain 'a.txt'", result)
	}
	if !contains(result, "subdir") {
		t.Errorf("result %q does not contain 'subdir'", result)
	}
	if !contains(result, `"total":2`) {
		t.Errorf("result %q does not contain total:2", result)
	}
}

func TestListDirTool_InvokableRun_Recursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "deep.txt"), []byte("deep"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewListDirTool()
	result, err := tool.InvokableRun(context.Background(), `{"path": "`+dir+`", "recursive": true}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, "deep.txt") {
		t.Errorf("result %q does not contain 'deep.txt'", result)
	}
}

func TestListDirTool_InvokableRun_Pattern(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := NewListDirTool()
	result, err := tool.InvokableRun(context.Background(), `{"path": "`+dir+`", "pattern": "*.go"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, "b.go") {
		t.Errorf("result %q does not contain 'b.go'", result)
	}
	if !contains(result, `"total":1`) {
		t.Errorf("expected total:1, got %q", result)
	}
}

func TestListDirTool_InvokableRun_EmptyPath(t *testing.T) {
	tool := NewListDirTool()
	_, err := tool.InvokableRun(context.Background(), `{"path": ""}`)
	if err == nil {
		t.Error("expected error for empty path")
	}
}

// --- GitTool tests ---

func TestGitTool_Info(t *testing.T) {
	tool := NewGitTool()
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "git" {
		t.Errorf("Name = %q, want %q", info.Name, "git")
	}
}

func TestGitTool_InvokableRun_Status(t *testing.T) {
	dir := t.TempDir()
	// Init a git repo
	initCmd := filepath.Join(dir, "init")
	_ = initCmd
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	// Change to the repo dir for the git status
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	tool := NewGitTool()
	result, err := tool.InvokableRun(context.Background(), `{"action": "status"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, `"exit_code":0`) {
		t.Errorf("expected exit_code:0 in result %q", result)
	}
}

func TestGitTool_InvokableRun_InvalidAction(t *testing.T) {
	tool := NewGitTool()
	_, err := tool.InvokableRun(context.Background(), `{"action": "invalid_action"}`)
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestGitTool_InvokableRun_EmptyAction(t *testing.T) {
	tool := NewGitTool()
	_, err := tool.InvokableRun(context.Background(), `{"action": ""}`)
	if err == nil {
		t.Error("expected error for empty action")
	}
}

// --- ActivateToolsTool tests ---

// mockActivator is a test double for ToolActivator.
type mockActivator struct {
	known     map[string]bool
	activated map[string][]string // sessionID → tool names
}

func newMockActivator(known []string) *mockActivator {
	m := &mockActivator{
		known:     make(map[string]bool, len(known)),
		activated: make(map[string][]string),
	}
	for _, n := range known {
		m.known[n] = true
	}
	return m
}

func (m *mockActivator) IsKnown(name string) bool { return m.known[name] }
func (m *mockActivator) Activate(sessionID, name string) bool {
	if !m.known[name] {
		return false
	}
	m.activated[sessionID] = append(m.activated[sessionID], name)
	return true
}

func TestActivateToolsTool_Info(t *testing.T) {
	activator := newMockActivator(nil)
	bus := events.NewBus(16)
	defer bus.Close()
	registry := NewToolRegistry(bus)
	defer registry.Close(context.Background())

	at := NewActivateToolsTool(activator, registry)
	info, err := at.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "activate_tools" {
		t.Errorf("Name = %q, want %q", info.Name, "activate_tools")
	}
}

func TestActivateToolsTool_ActivateKnown(t *testing.T) {
	activator := newMockActivator([]string{"search", "git"})
	bus := events.NewBus(16)
	defer bus.Close()
	registry := NewToolRegistry(bus)
	defer registry.Close(context.Background())

	// Register tools so we can get descriptions
	if err := registry.RegisterNative("search", NewSearchTool(), SearchManifest()); err != nil {
		t.Fatal(err)
	}

	at := NewActivateToolsTool(activator, registry)
	ctx := events.ContextWithSessionID(context.Background(), "sess1")

	result, err := at.InvokableRun(ctx, `{"names": ["search"]}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, `"name":"search"`) {
		t.Errorf("result %q does not contain activated search", result)
	}
	if len(activator.activated["sess1"]) != 1 || activator.activated["sess1"][0] != "search" {
		t.Errorf("activator.activated = %v, want [search]", activator.activated["sess1"])
	}
}

func TestActivateToolsTool_ActivateUnknown(t *testing.T) {
	activator := newMockActivator([]string{"search"})
	bus := events.NewBus(16)
	defer bus.Close()
	registry := NewToolRegistry(bus)
	defer registry.Close(context.Background())

	at := NewActivateToolsTool(activator, registry)
	ctx := events.ContextWithSessionID(context.Background(), "sess1")

	result, err := at.InvokableRun(ctx, `{"names": ["nonexistent"]}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !contains(result, "unknown tool") {
		t.Errorf("result %q does not contain error for unknown tool", result)
	}
}

func TestActivateToolsTool_NoSessionID(t *testing.T) {
	activator := newMockActivator([]string{"search"})
	bus := events.NewBus(16)
	defer bus.Close()
	registry := NewToolRegistry(bus)
	defer registry.Close(context.Background())

	at := NewActivateToolsTool(activator, registry)
	_, err := at.InvokableRun(context.Background(), `{"names": ["search"]}`)
	if err == nil {
		t.Error("expected error for missing session ID")
	}
}

func TestActivateToolsTool_EmptyNames(t *testing.T) {
	activator := newMockActivator([]string{"search"})
	bus := events.NewBus(16)
	defer bus.Close()
	registry := NewToolRegistry(bus)
	defer registry.Close(context.Background())

	at := NewActivateToolsTool(activator, registry)
	ctx := events.ContextWithSessionID(context.Background(), "sess1")

	_, err := at.InvokableRun(ctx, `{"names": []}`)
	if err == nil {
		t.Error("expected error for empty names")
	}
}
