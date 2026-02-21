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

const defaultCmdTimeout = 30 * time.Second

// CmdTool executes shell commands and returns stdout/stderr.
type CmdTool struct {
	timeout time.Duration
}

// NewCmdTool creates a new shell command tool.
func NewCmdTool(timeout time.Duration) *CmdTool {
	if timeout == 0 {
		timeout = defaultCmdTimeout
	}
	return &CmdTool{timeout: timeout}
}

// CmdManifest returns the plugin manifest for the cmd tool.
func CmdManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "cmd",
		Description: "Execute a shell command and return its output",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   true,
		Capabilities: CapabilitySet{
			Exec: true,
		},
		Tools: []ToolSpec{
			{
				Name:        "cmd",
				Description: "Execute a shell command. Returns stdout and stderr. Use for running system commands, scripts, or CLI tools.",
				Parameters: map[string]ParamSpec{
					"command": {
						Type:        "string",
						Description: "The shell command to execute",
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

type cmdInput struct {
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir"`
}

type cmdOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// Info returns the tool info for Eino registration.
func (t *CmdTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&CmdManifest().Tools[0]), nil
}

// InvokableRun executes the shell command.
func (t *CmdTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input cmdInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("cmd: parse input: %w", err)
	}
	if input.Command == "" {
		return "", fmt.Errorf("cmd: command is required")
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
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
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", fmt.Errorf("cmd: exec: %w", err)
		}
	}

	result := cmdOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("cmd: marshal result: %w", err)
	}
	return string(out), nil
}

var _ tool.InvokableTool = (*CmdTool)(nil)
