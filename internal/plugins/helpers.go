package plugins

import (
	"fmt"
	"path/filepath"
)

// resolveAbsWorkDir resolves a relative working directory to an absolute path.
// Returns the path unchanged if already absolute or empty.
func resolveAbsWorkDir(workDir, context string) (string, error) {
	if workDir == "" || filepath.IsAbs(workDir) {
		return workDir, nil
	}
	abs, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("%s: resolve work_dir: %w", context, err)
	}
	return abs, nil
}
