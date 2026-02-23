package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/events"
)

// sandboxToolType classifies a tool for sandbox validation.
type sandboxToolType string

const (
	sandboxExec       sandboxToolType = "exec"
	sandboxFilesystem sandboxToolType = "filesystem"
)

// SandboxGuard wraps a tool.InvokableTool with command and path validation.
// In autonomous mode it blocks destructive patterns and jails paths to the WorkDir.
// In interactive mode it passes through without checks.
type SandboxGuard struct {
	inner        tool.InvokableTool
	toolName     string
	toolType     sandboxToolType
	elevated     bool     // true for root_cmd — always blocked in autonomous mode
	allowedPaths []string // extra paths allowed outside WorkDir
}

// WrapSandbox wraps a tool with sandbox validation.
func WrapSandbox(t tool.InvokableTool, name string, tt sandboxToolType, elevated bool, allowedPaths []string) tool.InvokableTool {
	return &SandboxGuard{
		inner:        t,
		toolName:     name,
		toolType:     tt,
		elevated:     elevated,
		allowedPaths: allowedPaths,
	}
}

// Info delegates to the inner tool.
func (s *SandboxGuard) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return s.inner.Info(ctx)
}

// InvokableRun validates the tool call before delegating to the inner tool.
func (s *SandboxGuard) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// Only enforce in autonomous mode
	if !events.IsAutonomousContext(ctx) {
		return s.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	}

	// Elevated tools are unconditionally blocked in autonomous mode.
	// For the unified run_command tool, check the sudo flag dynamically.
	if s.elevated || (s.toolType == sandboxExec && IsSudo(argumentsInJSON)) {
		return "", fmt.Errorf("sandbox: tool %q is blocked in autonomous mode (elevated privileges)", s.toolName)
	}

	workDir := events.WorkDirFromContext(ctx)

	switch s.toolType {
	case sandboxExec:
		if err := s.validateExec(workDir, argumentsInJSON); err != nil {
			return "", err
		}
	case sandboxFilesystem:
		if err := s.validateFilesystem(workDir, argumentsInJSON); err != nil {
			return "", err
		}
	}

	return s.inner.InvokableRun(ctx, argumentsInJSON, opts...)
}

// validateExec checks a command execution tool call.
// Handles both raw-command tools (cmd, run_command with "command" field)
// and structured tools (git with "action"+"args" fields).
func (s *SandboxGuard) validateExec(workDir, argsJSON string) error {
	var args struct {
		Command    string `json:"command"`
		WorkingDir string `json:"working_dir"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Errorf("sandbox: %s: parse args: %w", s.toolName, err)
	}

	// 1. Denylist — only for tools with a raw command string
	if args.Command != "" {
		if rule := matchDestructivePattern(args.Command); rule != nil {
			return fmt.Errorf("sandbox: %s: blocked destructive command (%s)", s.toolName, rule.reason)
		}
	}

	// 2. Path jail — only when WorkDir is set
	if workDir == "" {
		return nil
	}

	// Check working_dir override
	if args.WorkingDir != "" {
		if err := validatePathInWorkDir(workDir, args.WorkingDir, s.allowedPaths); err != nil {
			return fmt.Errorf("sandbox: %s: working_dir %s", s.toolName, err)
		}
	}

	// Check paths in the raw command string
	if args.Command != "" {
		for _, p := range extractCommandPaths(args.Command) {
			if err := validatePathInWorkDir(workDir, p, s.allowedPaths); err != nil {
				return fmt.Errorf("sandbox: %s: command path %s", s.toolName, err)
			}
		}
	}

	// Check path-like fields in JSON args (covers structured tools like git
	// where paths live in nested objects, e.g. {"action":"add","args":{"paths":[...]}})
	for _, p := range extractToolPaths(argsJSON) {
		if err := validatePathInWorkDir(workDir, p, s.allowedPaths); err != nil {
			return fmt.Errorf("sandbox: %s: arg path %s", s.toolName, err)
		}
	}

	return nil
}

// validateFilesystem checks a filesystem tool call.
func (s *SandboxGuard) validateFilesystem(workDir, argsJSON string) error {
	if workDir == "" {
		return nil
	}

	for _, p := range extractToolPaths(argsJSON) {
		if err := validatePathInWorkDir(workDir, p, s.allowedPaths); err != nil {
			return fmt.Errorf("sandbox: %s: %s", s.toolName, err)
		}
	}

	return nil
}

// validatePathInWorkDir checks that a path resolves within the workDir or allowedPaths.
// It resolves symlinks (best-effort) to prevent symlink escapes.
func validatePathInWorkDir(workDir, path string, allowedPaths []string) error {
	cleanWorkDir := filepath.Clean(workDir)

	// Resolve symlinks on workDir itself for consistent comparison
	if real, err := filepath.EvalSymlinks(cleanWorkDir); err == nil {
		cleanWorkDir = real
	}

	// Resolve the path: if relative, join with workDir
	resolved := path
	if !filepath.IsAbs(resolved) {
		// Expand ~ to workDir (agent paths shouldn't use $HOME)
		if strings.HasPrefix(resolved, "~/") {
			resolved = filepath.Join(cleanWorkDir, resolved[2:])
		} else {
			resolved = filepath.Join(cleanWorkDir, resolved)
		}
	}
	resolved = filepath.Clean(resolved)

	// Best-effort symlink resolution — check the real target
	if real, err := evalSymlinksExisting(resolved); err == nil {
		resolved = real
	}

	// Check if within workDir
	if isUnder(resolved, cleanWorkDir) {
		return nil
	}

	// Check allowedPaths
	for _, ap := range allowedPaths {
		cleanAP := filepath.Clean(ap)
		if real, err := filepath.EvalSymlinks(cleanAP); err == nil {
			cleanAP = real
		}
		if isUnder(resolved, cleanAP) {
			return nil
		}
	}

	return fmt.Errorf("path %q is outside work directory %q", path, workDir)
}

// isUnder returns true if child is equal to or a descendant of parent.
func isUnder(child, parent string) bool {
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}

// evalSymlinksExisting resolves symlinks for the longest existing prefix of a path.
// For paths where intermediate directories exist but the leaf does not,
// it resolves what exists and appends the remaining components.
func evalSymlinksExisting(path string) (string, error) {
	real, err := filepath.EvalSymlinks(path)
	if err == nil {
		return real, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	// Walk up to find the longest existing ancestor
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	if dir == path {
		// Root or can't go higher
		return "", err
	}

	resolvedDir, err := evalSymlinksExisting(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

var _ tool.InvokableTool = (*SandboxGuard)(nil)
