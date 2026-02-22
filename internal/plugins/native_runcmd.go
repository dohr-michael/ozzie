package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const (
	defaultRunCmdTimeout = 30 * time.Second
	maxRunCmdTimeout     = 300 * time.Second
)

// RunCmdTool executes shell commands with configurable timeout.
type RunCmdTool struct{}

// NewRunCmdTool creates a new run_command tool.
func NewRunCmdTool() *RunCmdTool {
	return &RunCmdTool{}
}

// RunCmdManifest returns the plugin manifest for the run_command tool.
func RunCmdManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "run_command",
		Description: "Execute a shell command with configurable timeout",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   true,
		Capabilities: CapabilitySet{
			Exec: true,
		},
		Tools: []ToolSpec{
			{
				Name:        "run_command",
				Description: "Execute a shell command with configurable timeout. Returns stdout, stderr, and exit code.",
				Parameters: map[string]ParamSpec{
					"command": {
						Type:        "string",
						Description: "The shell command to execute",
						Required:    true,
					},
					"working_dir": {
						Type:        "string",
						Description: "Working directory for the command",
					},
					"timeout": {
						Type:        "integer",
						Description: "Timeout in seconds (default: 30, max: 300)",
					},
				},
				Dangerous: true,
			},
		},
	}
}

type runCmdInput struct {
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir"`
	Timeout    int    `json:"timeout"`
}

// Info returns the tool info for Eino registration.
func (t *RunCmdTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&RunCmdManifest().Tools[0]), nil
}

// InvokableRun executes the shell command.
func (t *RunCmdTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input runCmdInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("run_command: parse input: %w", err)
	}
	if input.Command == "" {
		return "", fmt.Errorf("run_command: command is required")
	}

	timeout := defaultRunCmdTimeout
	if input.Timeout > 0 {
		timeout = time.Duration(input.Timeout) * time.Second
		if timeout > maxRunCmdTimeout {
			timeout = maxRunCmdTimeout
		}
	}

	slog.Info("run_command: executing", "command", input.Command, "timeout", timeout)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", input.Command)
	if input.WorkingDir != "" {
		cmd.Dir = input.WorkingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("run_command: %w", ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", fmt.Errorf("run_command: exec: %w", err)
		}
	}

	result := cmdOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("run_command: marshal result: %w", err)
	}
	return string(out), nil
}

var _ tool.InvokableTool = (*RunCmdTool)(nil)
