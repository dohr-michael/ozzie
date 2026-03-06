package setup_wizard

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpProbeTimeout = 15 * time.Second

// probeMCPTools connects to an MCP server and returns the list of tool names.
func probeMCPTools(entry MCPServerEntry, envVars []MCPEnvVar) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mcpProbeTimeout)
	defer cancel()

	transport, err := buildProbeTransport(entry, envVars)
	if err != nil {
		return nil, fmt.Errorf("build transport: %w", err)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "ozzie-wizard",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer session.Close()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	names := make([]string, len(result.Tools))
	for i, t := range result.Tools {
		names[i] = t.Name
	}
	sort.Strings(names)
	return names, nil
}

// buildProbeTransport creates the MCP transport for probing.
func buildProbeTransport(entry MCPServerEntry, envVars []MCPEnvVar) (mcp.Transport, error) {
	switch entry.Transport {
	case "stdio":
		if entry.Command == "" {
			return nil, fmt.Errorf("stdio transport requires command")
		}
		cmd := exec.Command(entry.Command, entry.Args...)
		if len(envVars) > 0 {
			env := make([]string, 0, len(envVars))
			for _, ev := range envVars {
				if ev.Value != "" {
					env = append(env, ev.Name+"="+ev.Value)
				}
			}
			if len(env) > 0 {
				cmd.Env = env
			}
		}
		return &mcp.CommandTransport{Command: cmd}, nil

	case "sse":
		if entry.URL == "" {
			return nil, fmt.Errorf("sse transport requires url")
		}
		return &mcp.SSEClientTransport{Endpoint: entry.URL}, nil

	case "http":
		if entry.URL == "" {
			return nil, fmt.Errorf("http transport requires url")
		}
		return &mcp.StreamableClientTransport{Endpoint: entry.URL}, nil

	default:
		return nil, fmt.Errorf("unknown transport %q", entry.Transport)
	}
}
