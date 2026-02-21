package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// RootCmdTool executes commands with sudo privileges.
type RootCmdTool struct {
	timeout time.Duration
}

// NewRootCmdTool creates a new elevated command tool.
func NewRootCmdTool(timeout time.Duration) *RootCmdTool {
	if timeout == 0 {
		timeout = defaultCmdTimeout
	}
	return &RootCmdTool{timeout: timeout}
}

// RootCmdManifest returns the plugin manifest for the root_cmd tool.
func RootCmdManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "root_cmd",
		Description: "Execute a command with elevated (sudo) privileges",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   true,
		Capabilities: CapabilitySet{
			Exec:     true,
			Elevated: true,
		},
		Tools: []ToolSpec{
			{
				Name:        "root_cmd",
				Description: "Execute a command with sudo (root) privileges. Use only when elevated permissions are required.",
				Parameters: map[string]ParamSpec{
					"command": {
						Type:        "string",
						Description: "The command to execute with sudo",
						Required:    true,
					},
					"working_dir": {
						Type:        "string",
						Description: "Working directory for the command (optional)",
					},
				},
				Dangerous: true,
			},
		},
	}
}

type rootCmdInput struct {
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir"`
}

// Info returns the tool info for Eino registration.
func (t *RootCmdTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&RootCmdManifest().Tools[0]), nil
}

// InvokableRun executes the command with sudo.
func (t *RootCmdTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input rootCmdInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("root_cmd: parse input: %w", err)
	}
	if input.Command == "" {
		return "", fmt.Errorf("root_cmd: command is required")
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "sh", "-c", input.Command)
	if input.WorkingDir != "" {
		cmd.Dir = input.WorkingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", fmt.Errorf("root_cmd: exec: %w", err)
		}
	}

	result := cmdOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("root_cmd: marshal result: %w", err)
	}
	return string(out), nil
}

var _ tool.InvokableTool = (*RootCmdTool)(nil)
