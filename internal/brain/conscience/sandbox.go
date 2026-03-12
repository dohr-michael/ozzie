package conscience

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dohr-michael/ozzie/internal/brain"
	"github.com/dohr-michael/ozzie/internal/core/events"
)

// sandboxToolType classifies a tool for sandbox validation.
type sandboxToolType string

const (
	SandboxExec       sandboxToolType = "exec"
	SandboxFilesystem sandboxToolType = "filesystem"
)

// SandboxGuard wraps a brain.Tool with command and path validation.
// In autonomous mode it blocks destructive patterns and jails paths to the WorkDir.
// In interactive mode it passes through without checks.
type SandboxGuard struct {
	inner        brain.Tool
	toolName     string
	toolType     sandboxToolType
	elevated     bool     // true for root_cmd — always blocked in autonomous mode
	allowedPaths []string // extra paths allowed outside WorkDir
}

// WrapSandbox wraps a tool with sandbox validation.
func WrapSandbox(t brain.Tool, name string, tt sandboxToolType, elevated bool, allowedPaths []string) brain.Tool {
	return &SandboxGuard{
		inner:        t,
		toolName:     name,
		toolType:     tt,
		elevated:     elevated,
		allowedPaths: allowedPaths,
	}
}

// Info delegates to the inner tool.
func (s *SandboxGuard) Info(ctx context.Context) (*brain.ToolInfo, error) {
	return s.inner.Info(ctx)
}

// Run validates the tool call before delegating to the inner tool.
func (s *SandboxGuard) Run(ctx context.Context, argumentsInJSON string) (string, error) {
	// Only enforce in autonomous mode
	if !events.IsAutonomousContext(ctx) {
		return s.inner.Run(ctx, argumentsInJSON)
	}

	// Elevated tools are unconditionally blocked in autonomous mode.
	// For the unified run_command tool, check the sudo flag dynamically.
	if s.elevated || (s.toolType == SandboxExec && isSudo(argumentsInJSON)) {
		return "", fmt.Errorf("sandbox: tool %q is blocked in autonomous mode (elevated privileges)", s.toolName)
	}

	workDir := events.WorkDirFromContext(ctx)

	switch s.toolType {
	case SandboxExec:
		if err := s.validateExec(workDir, argumentsInJSON); err != nil {
			return "", err
		}
	case SandboxFilesystem:
		if err := s.validateFilesystem(workDir, argumentsInJSON); err != nil {
			return "", err
		}
	}

	return s.inner.Run(ctx, argumentsInJSON)
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

	// 1. Denylist — AST-based validation for raw command strings
	if args.Command != "" {
		if err := validateCommandAST(args.Command); err != nil {
			return fmt.Errorf("sandbox: %s: %w", s.toolName, err)
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

	// Check paths in the raw command string (AST-based)
	if args.Command != "" {
		for _, p := range extractCommandPathsAST(args.Command) {
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
	if real, err := EvalSymlinksExisting(resolved); err == nil {
		resolved = real
	}

	// Check if within workDir
	if IsUnder(resolved, cleanWorkDir) {
		return nil
	}

	// Check allowedPaths
	for _, ap := range allowedPaths {
		cleanAP := filepath.Clean(ap)
		if real, err := filepath.EvalSymlinks(cleanAP); err == nil {
			cleanAP = real
		}
		if IsUnder(resolved, cleanAP) {
			return nil
		}
	}

	return fmt.Errorf("path %q is outside work directory %q", path, workDir)
}

// IsUnder returns true if child is equal to or a descendant of parent.
func IsUnder(child, parent string) bool {
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}

// EvalSymlinksExisting resolves symlinks for the longest existing prefix of a path.
// For paths where intermediate directories exist but the leaf does not,
// it resolves what exists and appends the remaining components.
func EvalSymlinksExisting(path string) (string, error) {
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

	resolvedDir, err := EvalSymlinksExisting(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

// isSudo extracts the sudo flag from a tool call's JSON arguments.
// Used by the sandbox to apply elevated restrictions dynamically.
func isSudo(argumentsInJSON string) bool {
	var args struct {
		Sudo bool `json:"sudo"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return false
	}
	return args.Sudo
}

var _ brain.Tool = (*SandboxGuard)(nil)
