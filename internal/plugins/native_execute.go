package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/events"
)

// mergeEnv returns a copy of base with extra key=value pairs appended.
// Used by exec tools to inject task environment variables.
func mergeEnv(base []string, extra map[string]string) []string {
	env := make([]string, len(base), len(base)+len(extra))
	copy(env, base)
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

// applyTaskEnv sets cmd.Env from the task environment in the context.
// If no task env is set, cmd.Env stays nil (inherits parent process env).
func applyTaskEnv(ctx context.Context, cmd *exec.Cmd) {
	if env := events.TaskEnvFromContext(ctx); len(env) > 0 {
		cmd.Env = mergeEnv(os.Environ(), env)
	}
}

const (
	defaultExecuteTimeout = 30 * time.Second
	maxExecuteTimeout     = 300 * time.Second
)

// ExecuteTool executes shell commands with optional sudo and configurable timeout.
// It unifies the former cmd, root_cmd, and run_command tools.
type ExecuteTool struct{}

// NewExecuteTool creates a new unified execution tool.
func NewExecuteTool() *ExecuteTool {
	return &ExecuteTool{}
}

// ExecuteManifest returns the plugin manifest for the run_command tool.
func ExecuteManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "run_command",
		Description: "Execute a shell command with optional sudo and configurable timeout",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   true,
		Capabilities: CapabilitySet{
			Exec: true,
		},
		Tools: []ToolSpec{
			{
				Name:        "run_command",
				Description: "Execute a shell command. Returns stdout, stderr, and exit code. Use sudo=true only when elevated permissions are required.",
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
					"sudo": {
						Type:        "boolean",
						Description: "Run with sudo (root) privileges (default: false)",
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

type executeInput struct {
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir"`
	Sudo       bool   `json:"sudo"`
	Timeout    int    `json:"timeout"`
}

type executeOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// Info returns the tool info for Eino registration.
func (t *ExecuteTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ExecuteManifest().Tools[0]), nil
}

// InvokableRun executes the shell command.
func (t *ExecuteTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input executeInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("run_command: parse input: %w", err)
	}
	if input.Command == "" {
		return "", fmt.Errorf("run_command: command is required")
	}

	timeout := defaultExecuteTimeout
	if input.Timeout > 0 {
		timeout = time.Duration(input.Timeout) * time.Second
		if timeout > maxExecuteTimeout {
			timeout = maxExecuteTimeout
		}
	}

	slog.Info("run_command: executing", "command", input.Command, "sudo", input.Sudo, "timeout", timeout)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if input.Sudo {
		cmd = exec.CommandContext(ctx, "sudo", "sh", "-c", input.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", input.Command)
	}

	if input.WorkingDir != "" {
		cmd.Dir = input.WorkingDir
	} else if wd := events.WorkDirFromContext(ctx); wd != "" {
		cmd.Dir = wd
	}
	applyTaskEnv(ctx, cmd)

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

	result := executeOutput{
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

// IsSudo extracts the sudo flag from a tool call's JSON arguments.
// Used by the sandbox to apply elevated restrictions dynamically.
func IsSudo(argumentsInJSON string) bool {
	var args struct {
		Sudo bool `json:"sudo"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return false
	}
	return args.Sudo
}

var _ tool.InvokableTool = (*ExecuteTool)(nil)
