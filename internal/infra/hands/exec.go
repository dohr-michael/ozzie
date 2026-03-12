package hands

import (
	"bytes"
	"context"
	"os/exec"
)

// ExecResult holds the output of a command execution.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// execCommand runs cmd and captures stdout, stderr, and exit code.
// It does NOT wrap errors with context — callers are responsible for that.
func execCommand(ctx context.Context, cmd *exec.Cmd) (ExecResult, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ExecResult{}, ctx.Err()
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return ExecResult{}, err
		}
	}

	return ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}
