package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/plugins"
)

func TestToolSpecToMCPTool(t *testing.T) {
	spec := &plugins.ToolSpec{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]plugins.ParamSpec{
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
			"mode": {
				Type:        "string",
				Description: "The mode",
				Required:    true,
				Enum:        []string{"fast", "slow"},
			},
		},
	}

	mcpTool := toolSpecToMCPTool(spec)

	if mcpTool.Name != "test_tool" {
		t.Errorf("Name = %q, want %q", mcpTool.Name, "test_tool")
	}
	if mcpTool.Description != "A test tool" {
		t.Errorf("Description = %q, want %q", mcpTool.Description, "A test tool")
	}

	// Verify InputSchema is a proper JSON Schema object
	schemaBytes, err := json.Marshal(mcpTool.InputSchema)
	if err != nil {
		t.Fatalf("marshal InputSchema: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("unmarshal InputSchema: %v", err)
	}

	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want %q", schema["type"], "object")
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties not a map")
	}
	if len(props) != 3 {
		t.Errorf("schema properties len = %d, want 3", len(props))
	}

	// Check required field (sorted)
	req, ok := schema["required"].([]any)
	if !ok {
		t.Fatal("schema required not an array")
	}
	if len(req) != 2 {
		t.Fatalf("schema required len = %d, want 2", len(req))
	}
	// Sorted: mode, name
	if req[0] != "mode" || req[1] != "name" {
		t.Errorf("schema required = %v, want [mode, name]", req)
	}

	// Check enum on mode
	modeProp, ok := props["mode"].(map[string]any)
	if !ok {
		t.Fatal("mode property not a map")
	}
	enumVal, ok := modeProp["enum"].([]any)
	if !ok {
		t.Fatal("mode enum not an array")
	}
	if len(enumVal) != 2 {
		t.Errorf("mode enum len = %d, want 2", len(enumVal))
	}
}

func TestToolSpecToMCPTool_NoParams(t *testing.T) {
	spec := &plugins.ToolSpec{
		Name:        "simple",
		Description: "A simple tool",
		Parameters:  map[string]plugins.ParamSpec{},
	}

	mcpTool := toolSpecToMCPTool(spec)

	schemaBytes, err := json.Marshal(mcpTool.InputSchema)
	if err != nil {
		t.Fatalf("marshal InputSchema: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("unmarshal InputSchema: %v", err)
	}

	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want %q", schema["type"], "object")
	}
	// No required field when no required params
	if _, ok := schema["required"]; ok {
		t.Error("schema should not have required field when no params are required")
	}
}

func TestNewMCPServer_AllTools(t *testing.T) {
	bus := events.NewBus(16)
	defer bus.Close()

	registry := plugins.NewToolRegistry(bus)
	defer registry.Close(context.Background())

	if err := registry.RegisterNative("cmd", plugins.NewCmdTool(0), plugins.CmdManifest()); err != nil {
		t.Fatal(err)
	}

	server := NewMCPServer(registry, "")
	if server == nil {
		t.Fatal("NewMCPServer returned nil")
	}
}

func TestNewMCPServer_WithFilter(t *testing.T) {
	bus := events.NewBus(16)
	defer bus.Close()

	registry := plugins.NewToolRegistry(bus)
	defer registry.Close(context.Background())

	if err := registry.RegisterNative("cmd", plugins.NewCmdTool(0), plugins.CmdManifest()); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterNative("root_cmd", plugins.NewRootCmdTool(0), plugins.RootCmdManifest()); err != nil {
		t.Fatal(err)
	}

	// Filter by tool name
	server := NewMCPServer(registry, "cmd")
	if server == nil {
		t.Fatal("NewMCPServer with filter returned nil")
	}
}

func TestMatchesFilter(t *testing.T) {
	bus := events.NewBus(16)
	defer bus.Close()

	registry := plugins.NewToolRegistry(bus)
	defer registry.Close(context.Background())

	if err := registry.RegisterNative("cmd", plugins.NewCmdTool(0), plugins.CmdManifest()); err != nil {
		t.Fatal(err)
	}

	// Direct tool name match
	if !matchesFilter(registry, "cmd", "cmd") {
		t.Error("matchesFilter(cmd, cmd) = false, want true")
	}

	// Non-match
	if matchesFilter(registry, "cmd", "other") {
		t.Error("matchesFilter(cmd, other) = true, want false")
	}
}
