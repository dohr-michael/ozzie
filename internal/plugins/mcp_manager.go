package plugins

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPManager manages the lifecycle of external MCP client sessions.
type MCPManager struct {
	clients  map[string]*mcp.Client
	sessions map[string]*mcp.ClientSession
	mu       sync.Mutex
}

// NewMCPManager creates a new MCP manager.
func NewMCPManager() *MCPManager {
	return &MCPManager{
		clients:  make(map[string]*mcp.Client),
		sessions: make(map[string]*mcp.ClientSession),
	}
}

// Connect establishes a connection to an MCP server and returns the session.
func (m *MCPManager) Connect(ctx context.Context, name string, cfg *config.MCPServerConfig) (*mcp.ClientSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[name]; ok {
		return session, nil
	}

	transport, err := buildTransport(cfg)
	if err != nil {
		return nil, fmt.Errorf("mcp %q: build transport: %w", name, err)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "ozzie",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp %q: connect: %w", name, err)
	}

	m.clients[name] = client
	m.sessions[name] = session
	return session, nil
}

// Session returns the session for a given server name, or nil.
func (m *MCPManager) Session(name string) *mcp.ClientSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[name]
}

// Close gracefully shuts down all MCP sessions.
func (m *MCPManager) Close(_ context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, session := range m.sessions {
		if err := session.Close(); err != nil {
			slog.Warn("mcp session close error", "server", name, "error", err)
		}
	}
	m.sessions = make(map[string]*mcp.ClientSession)
	m.clients = make(map[string]*mcp.Client)
}

// buildTransport creates the appropriate MCP transport based on config.
func buildTransport(cfg *config.MCPServerConfig) (mcp.Transport, error) {
	switch cfg.Transport {
	case "stdio":
		if cfg.Command == "" {
			return nil, fmt.Errorf("stdio transport requires command")
		}
		cmd := exec.Command(cfg.Command, cfg.Args...)
		// Only pass declared env vars — do NOT inherit os.Environ()
		if len(cfg.Env) > 0 {
			env := make([]string, 0, len(cfg.Env))
			for k, v := range cfg.Env {
				env = append(env, k+"="+v)
			}
			cmd.Env = env
		}
		return &mcp.CommandTransport{Command: cmd}, nil

	case "sse":
		if cfg.URL == "" {
			return nil, fmt.Errorf("sse transport requires url")
		}
		return &mcp.SSEClientTransport{Endpoint: cfg.URL}, nil

	case "http":
		if cfg.URL == "" {
			return nil, fmt.Errorf("http transport requires url")
		}
		return &mcp.StreamableClientTransport{Endpoint: cfg.URL}, nil

	default:
		return nil, fmt.Errorf("unknown transport %q (expected stdio, sse, or http)", cfg.Transport)
	}
}
