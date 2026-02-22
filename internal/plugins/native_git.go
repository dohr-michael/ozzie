package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const defaultGitTimeout = 15 * time.Second

// GitTool provides git operations.
type GitTool struct{}

// NewGitTool creates a new git tool.
func NewGitTool() *GitTool {
	return &GitTool{}
}

// GitManifest returns the plugin manifest for the git tool.
func GitManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "git",
		Description: "Execute git operations",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   true,
		Capabilities: CapabilitySet{
			Exec: true,
		},
		Tools: []ToolSpec{
			{
				Name:        "git",
				Description: "Execute git operations: status, diff, log, add, commit, branch, checkout.",
				Parameters: map[string]ParamSpec{
					"action": {
						Type:        "string",
						Description: "Git action to perform",
						Required:    true,
						Enum:        []string{"status", "diff", "log", "add", "commit", "branch", "checkout"},
					},
					"args": {
						Type:        "object",
						Description: "Action-specific arguments (JSON object)",
					},
				},
				Dangerous: true,
			},
		},
	}
}

type gitInput struct {
	Action string          `json:"action"`
	Args   json.RawMessage `json:"args"`
}

type gitResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
}

// Action-specific arg structs
type gitStatusArgs struct {
	Path string `json:"path"`
}

type gitDiffArgs struct {
	Path   string `json:"path"`
	Staged bool   `json:"staged"`
}

type gitLogArgs struct {
	Path string `json:"path"`
	Max  int    `json:"max"`
}

type gitAddArgs struct {
	Paths []string `json:"paths"`
}

type gitCommitArgs struct {
	Message string `json:"message"`
}

type gitBranchArgs struct {
	Name string `json:"name"`
	List bool   `json:"list"`
}

type gitCheckoutArgs struct {
	Ref string `json:"ref"`
}

// Info returns the tool info for Eino registration.
func (t *GitTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&GitManifest().Tools[0]), nil
}

// InvokableRun executes the git action.
func (t *GitTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input gitInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("git: parse input: %w", err)
	}
	if input.Action == "" {
		return "", fmt.Errorf("git: action is required")
	}

	var result gitResult
	var err error

	switch input.Action {
	case "status":
		result, err = gitStatus(ctx, input.Args)
	case "diff":
		result, err = gitDiff(ctx, input.Args)
	case "log":
		result, err = gitLog(ctx, input.Args)
	case "add":
		result, err = gitAdd(ctx, input.Args)
	case "commit":
		result, err = gitCommit(ctx, input.Args)
	case "branch":
		result, err = gitBranch(ctx, input.Args)
	case "checkout":
		result, err = gitCheckout(ctx, input.Args)
	default:
		return "", fmt.Errorf("git: unknown action %q", input.Action)
	}
	if err != nil {
		return "", err
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("git: marshal result: %w", err)
	}
	return string(out), nil
}

func gitStatus(ctx context.Context, rawArgs json.RawMessage) (gitResult, error) {
	var args gitStatusArgs
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return gitResult{}, fmt.Errorf("git status: parse args: %w", err)
		}
	}
	cmdArgs := []string{"status", "--porcelain"}
	if args.Path != "" {
		cmdArgs = append(cmdArgs, args.Path)
	}
	return execGit(ctx, "", cmdArgs...)
}

func gitDiff(ctx context.Context, rawArgs json.RawMessage) (gitResult, error) {
	var args gitDiffArgs
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return gitResult{}, fmt.Errorf("git diff: parse args: %w", err)
		}
	}
	cmdArgs := []string{"diff"}
	if args.Staged {
		cmdArgs = append(cmdArgs, "--staged")
	}
	if args.Path != "" {
		cmdArgs = append(cmdArgs, args.Path)
	}
	return execGit(ctx, "", cmdArgs...)
}

func gitLog(ctx context.Context, rawArgs json.RawMessage) (gitResult, error) {
	var args gitLogArgs
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return gitResult{}, fmt.Errorf("git log: parse args: %w", err)
		}
	}
	max := args.Max
	if max <= 0 {
		max = 10
	}
	if max > 100 {
		max = 100
	}
	cmdArgs := []string{"log", "--oneline", "-" + strconv.Itoa(max)}
	if args.Path != "" {
		cmdArgs = append(cmdArgs, "--", args.Path)
	}
	return execGit(ctx, "", cmdArgs...)
}

func gitAdd(ctx context.Context, rawArgs json.RawMessage) (gitResult, error) {
	var args gitAddArgs
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return gitResult{}, fmt.Errorf("git add: parse args: %w", err)
		}
	}
	if len(args.Paths) == 0 {
		return gitResult{}, fmt.Errorf("git add: paths are required")
	}
	cmdArgs := append([]string{"add"}, args.Paths...)
	return execGit(ctx, "", cmdArgs...)
}

func gitCommit(ctx context.Context, rawArgs json.RawMessage) (gitResult, error) {
	var args gitCommitArgs
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return gitResult{}, fmt.Errorf("git commit: parse args: %w", err)
		}
	}
	if args.Message == "" {
		return gitResult{}, fmt.Errorf("git commit: message is required")
	}
	return execGit(ctx, "", "commit", "-m", args.Message)
}

func gitBranch(ctx context.Context, rawArgs json.RawMessage) (gitResult, error) {
	var args gitBranchArgs
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return gitResult{}, fmt.Errorf("git branch: parse args: %w", err)
		}
	}
	if args.List || args.Name == "" {
		return execGit(ctx, "", "branch", "-a")
	}
	return execGit(ctx, "", "branch", args.Name)
}

func gitCheckout(ctx context.Context, rawArgs json.RawMessage) (gitResult, error) {
	var args gitCheckoutArgs
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return gitResult{}, fmt.Errorf("git checkout: parse args: %w", err)
		}
	}
	if args.Ref == "" {
		return gitResult{}, fmt.Errorf("git checkout: ref is required")
	}
	return execGit(ctx, "", "checkout", args.Ref)
}

// execGit runs a git command and returns the output.
func execGit(ctx context.Context, dir string, args ...string) (gitResult, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultGitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return gitResult{}, fmt.Errorf("git: exec: %w", err)
		}
	}

	output := stdout.String()
	if output == "" {
		output = stderr.String()
	}

	return gitResult{
		Output:   output,
		ExitCode: exitCode,
	}, nil
}

var _ tool.InvokableTool = (*GitTool)(nil)
