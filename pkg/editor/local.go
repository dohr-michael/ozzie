package editor

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// skipDirs are directories skipped during ListDir traversal.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".hg":          true,
}

// LocalBackend implements Backend using the local filesystem.
type LocalBackend struct{}

func (LocalBackend) ReadFile(_ context.Context, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (LocalBackend) WriteFile(_ context.Context, path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dirs: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func (LocalBackend) IsDirectory(_ context.Context, path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("path does not exist: %s", path)
		}
		return false, err
	}
	return info.IsDir(), nil
}

func (LocalBackend) Exists(_ context.Context, path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (LocalBackend) ListDir(_ context.Context, path string, maxDepth int) ([]string, error) {
	var entries []string
	root := filepath.Clean(path)

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}

		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}

		// Enforce max depth
		depth := strings.Count(rel, string(filepath.Separator)) + 1
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden and common noise directories
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if strings.HasPrefix(d.Name(), ".") && d.IsDir() {
			return filepath.SkipDir
		}

		suffix := ""
		if d.IsDir() {
			suffix = "/"
		}
		entries = append(entries, rel+suffix)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

var _ Backend = LocalBackend{}
