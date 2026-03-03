package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---------------------------------------------------------------------------
// mcpToolToToolSpec tests
// ---------------------------------------------------------------------------

func TestMCPToolToToolSpec(t *testing.T) {
	tool := &mcp.Tool{
		Name:        "search",
		Description: "Search for items",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Max results",
				},
			},
			"required": []any{"query"},
		},
	}

	spec := mcpToolToToolSpec("github", tool)

	if spec.Name != "github__search" {
		t.Errorf("name = %q, want github__search", spec.Name)
	}
	if spec.Description != "Search for items" {
		t.Errorf("description = %q, want 'Search for items'", spec.Description)
	}
	if len(spec.Parameters) != 2 {
		t.Fatalf("parameters count = %d, want 2", len(spec.Parameters))
	}

	query := spec.Parameters["query"]
	if query.Type != "string" {
		t.Errorf("query.Type = %q, want string", query.Type)
	}
	if !query.Required {
		t.Error("query.Required = false, want true")
	}

	limit := spec.Parameters["limit"]
	if limit.Type != "integer" {
		t.Errorf("limit.Type = %q, want integer", limit.Type)
	}
	if limit.Required {
		t.Error("limit.Required = true, want false")
	}
}

func TestMCPToolToToolSpec_Empty(t *testing.T) {
	tool := &mcp.Tool{
		Name:        "noop",
		Description: "Does nothing",
	}

	spec := mcpToolToToolSpec("test", tool)

	if spec.Name != "test__noop" {
		t.Errorf("name = %q, want test__noop", spec.Name)
	}
	if len(spec.Parameters) != 0 {
		t.Errorf("parameters count = %d, want 0", len(spec.Parameters))
	}
}

func TestMCPToolToToolSpec_Nested(t *testing.T) {
	tool := &mcp.Tool{
		Name: "create",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"config": map[string]any{
					"type":        "object",
					"description": "Configuration object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
						"tags": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
					},
					"required": []any{"name"},
				},
			},
		},
	}

	spec := mcpToolToToolSpec("srv", tool)

	cfg, ok := spec.Parameters["config"]
	if !ok {
		t.Fatal("missing config parameter")
	}
	if cfg.Type != "object" {
		t.Errorf("config.Type = %q, want object", cfg.Type)
	}
	if len(cfg.Properties) != 2 {
		t.Fatalf("config.Properties count = %d, want 2", len(cfg.Properties))
	}

	name := cfg.Properties["name"]
	if name.Type != "string" {
		t.Errorf("config.name.Type = %q, want string", name.Type)
	}
	if !name.Required {
		t.Error("config.name.Required = false, want true")
	}

	tags := cfg.Properties["tags"]
	if tags.Type != "array" {
		t.Errorf("config.tags.Type = %q, want array", tags.Type)
	}
	if tags.Items == nil || tags.Items.Type != "string" {
		t.Error("config.tags.Items should be string type")
	}
}

// ---------------------------------------------------------------------------
// MCPTool.InvokableRun tests (using InMemoryTransport)
// ---------------------------------------------------------------------------

// testMCPSetup creates a test MCP server with tools and returns a connected client session.
func testMCPSetup(t *testing.T, setupServer func(server *mcp.Server)) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0"}, nil)
	setupServer(server)

	t1, t2 := mcp.NewInMemoryTransports()

	_, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	t.Cleanup(func() { session.Close() })
	return session
}

func TestMCPTool_InvokableRun(t *testing.T) {
	session := testMCPSetup(t, func(server *mcp.Server) {
		server.AddTool(
			&mcp.Tool{
				Name:        "echo",
				Description: "Echo input",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
			},
			func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				var args struct{ Text string }
				if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
					return nil, err
				}
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: "echo: " + args.Text}},
				}, nil
			},
		)
	})

	mcpTool := &MCPTool{
		serverName: "test",
		toolName:   "echo",
		session:    session,
		timeout:    5 * time.Second,
	}

	result, err := mcpTool.InvokableRun(context.Background(), `{"text":"hello"}`)
	if err != nil {
		t.Fatalf("InvokableRun error: %v", err)
	}
	if result != "echo: hello" {
		t.Errorf("result = %q, want 'echo: hello'", result)
	}
}

func TestMCPTool_InvokableRun_Error(t *testing.T) {
	session := testMCPSetup(t, func(server *mcp.Server) {
		server.AddTool(
			&mcp.Tool{
				Name:        "fail",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
			func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: "something went wrong"}},
				}, nil
			},
		)
	})

	mcpTool := &MCPTool{
		serverName: "test",
		toolName:   "fail",
		session:    session,
		timeout:    5 * time.Second,
	}

	_, err := mcpTool.InvokableRun(context.Background(), `{}`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != `mcp test__fail: tool error: something went wrong` {
		t.Errorf("error = %q", got)
	}
}

func TestMCPTool_InvokableRun_Timeout(t *testing.T) {
	session := testMCPSetup(t, func(server *mcp.Server) {
		server.AddTool(
			&mcp.Tool{
				Name:        "slow",
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
			func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				select {
				case <-time.After(5 * time.Second):
					return &mcp.CallToolResult{
						Content: []mcp.Content{&mcp.TextContent{Text: "done"}},
					}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		)
	})

	mcpTool := &MCPTool{
		serverName: "test",
		toolName:   "slow",
		session:    session,
		timeout:    100 * time.Millisecond,
	}

	_, err := mcpTool.InvokableRun(context.Background(), `{}`)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// ---------------------------------------------------------------------------
// MCPManager tests
// ---------------------------------------------------------------------------

func TestMCPManager_ConnectClose(t *testing.T) {
	ctx := context.Background()

	// Create a test server
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0"}, nil)
	server.AddTool(
		&mcp.Tool{
			Name:        "ping",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "pong"}},
			}, nil
		},
	)

	t1, t2 := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	manager := NewMCPManager()

	// We can't use manager.Connect directly because it creates its own transport.
	// Instead, manually wire up the session.
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	manager.clients["test"] = client
	manager.sessions["test"] = session

	if s := manager.Session("test"); s == nil {
		t.Error("Session returned nil for existing server")
	}
	if s := manager.Session("nonexistent"); s != nil {
		t.Error("Session returned non-nil for nonexistent server")
	}

	// Close should not panic
	manager.Close(ctx)

	if len(manager.sessions) != 0 {
		t.Error("sessions should be empty after Close")
	}
}

// ---------------------------------------------------------------------------
// SetupMCPServers tests (tool filtering)
// ---------------------------------------------------------------------------

func TestSetupMCPServers_AllowedDenied(t *testing.T) {
	tests := []struct {
		name         string
		allowed      []string
		denied       []string
		serverTools  []string
		wantFiltered []string // tools that should NOT be registered
		wantAllowed  []string // tools that SHOULD be registered
	}{
		{
			name:        "no filters — all allowed",
			serverTools: []string{"a", "b", "c"},
			wantAllowed: []string{"a", "b", "c"},
		},
		{
			name:         "denied takes priority",
			denied:       []string{"b"},
			serverTools:  []string{"a", "b", "c"},
			wantAllowed:  []string{"a", "c"},
			wantFiltered: []string{"b"},
		},
		{
			name:         "allowed restricts",
			allowed:      []string{"a"},
			serverTools:  []string{"a", "b", "c"},
			wantAllowed:  []string{"a"},
			wantFiltered: []string{"b", "c"},
		},
		{
			name:         "denied overrides allowed",
			allowed:      []string{"a", "b"},
			denied:       []string{"b"},
			serverTools:  []string{"a", "b", "c"},
			wantAllowed:  []string{"a"},
			wantFiltered: []string{"b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deniedSet := make(map[string]bool, len(tt.denied))
			for _, d := range tt.denied {
				deniedSet[d] = true
			}
			allowedSet := make(map[string]bool, len(tt.allowed))
			for _, a := range tt.allowed {
				allowedSet[a] = true
			}

			for _, toolName := range tt.wantAllowed {
				if !isToolAllowed(toolName, allowedSet, deniedSet) {
					t.Errorf("tool %q should be allowed", toolName)
				}
			}
			for _, toolName := range tt.wantFiltered {
				if isToolAllowed(toolName, allowedSet, deniedSet) {
					t.Errorf("tool %q should be filtered", toolName)
				}
			}
		})
	}
}

// TestSetupMCPServers_Integration tests the full setup flow with a real MCP server.
func TestSetupMCPServers_Integration(t *testing.T) {
	ctx := context.Background()

	// Create a test MCP server with two tools
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0"}, nil)
	server.AddTool(
		&mcp.Tool{
			Name:        "echo",
			Description: "Echo tool",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
		},
		func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args struct{ Text string }
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "echo: " + args.Text}},
			}, nil
		},
	)
	server.AddTool(
		&mcp.Tool{
			Name:        "add",
			Description: "Add numbers",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"]}`),
		},
		func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args struct {
				A float64 `json:"a"`
				B float64 `json:"b"`
			}
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("%.0f", args.A+args.B)}},
			}, nil
		},
	)

	t1, t2 := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	// Create a registry and manually wire the MCP manager
	bus := events.NewBus(64)
	defer bus.Close()

	registry := NewToolRegistry(bus)
	defer registry.Close(ctx)

	manager := NewMCPManager()
	registry.mcpManager = manager

	// Manually connect client (bypassing buildTransport which needs real stdio/sse/http)
	client := mcp.NewClient(&mcp.Implementation{Name: "ozzie", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	manager.clients["test"] = client
	manager.sessions["test"] = session

	// List and register tools
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	timeout := 5 * time.Second
	for _, mcpTool := range toolsResult.Tools {
		prefixed := "test__" + mcpTool.Name
		proxy := &MCPTool{
			serverName: "test",
			toolName:   mcpTool.Name,
			session:    session,
			mcpTool:    mcpTool,
			timeout:    timeout,
		}
		dangerous := true
		manifest := mcpToolManifest("test", mcpTool, dangerous)
		if err := registry.RegisterNative(prefixed, proxy, resolvedNativeManifest(manifest)); err != nil {
			t.Fatalf("register %s: %v", prefixed, err)
		}
	}

	// Verify tools are registered
	if registry.Tool("test__echo") == nil {
		t.Error("test__echo not registered")
	}
	if registry.Tool("test__add") == nil {
		t.Error("test__add not registered")
	}

	// Test calling echo
	echoResult, err := registry.Tool("test__echo").InvokableRun(ctx, `{"text":"world"}`)
	if err != nil {
		t.Fatalf("echo call: %v", err)
	}
	if echoResult != "echo: world" {
		t.Errorf("echo = %q, want 'echo: world'", echoResult)
	}

	// Test calling add
	addResult, err := registry.Tool("test__add").InvokableRun(ctx, `{"a":2,"b":3}`)
	if err != nil {
		t.Fatalf("add call: %v", err)
	}
	if addResult != "5" {
		t.Errorf("add = %q, want '5'", addResult)
	}

	// Verify manifest metadata
	manifest := registry.Manifest("test__echo")
	if manifest == nil {
		t.Fatal("manifest not found for test__echo")
	}
	if manifest.Provider != "mcp" {
		t.Errorf("provider = %q, want mcp", manifest.Provider)
	}
	if !manifest.Dangerous {
		t.Error("dangerous = false, want true")
	}
}

func TestMCPServerConfig_IsDangerous(t *testing.T) {
	// nil → true (default)
	cfg := &config.MCPServerConfig{}
	if !cfg.IsDangerous() {
		t.Error("nil dangerous should default to true")
	}

	// explicit true
	tr := true
	cfg.Dangerous = &tr
	if !cfg.IsDangerous() {
		t.Error("explicit true should be true")
	}

	// explicit false
	fa := false
	cfg.Dangerous = &fa
	if cfg.IsDangerous() {
		t.Error("explicit false should be false")
	}
}
