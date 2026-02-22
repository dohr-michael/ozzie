package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const maxListEntries = 1000

// ListDirTool lists directory contents.
type ListDirTool struct{}

// NewListDirTool creates a new list_dir tool.
func NewListDirTool() *ListDirTool {
	return &ListDirTool{}
}

// ListDirManifest returns the plugin manifest for the list_dir tool.
func ListDirManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "list_dir",
		Description: "List directory contents",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Capabilities: CapabilitySet{
			Filesystem: &FSCapability{ReadOnly: true},
		},
		Tools: []ToolSpec{
			{
				Name:        "list_dir",
				Description: "List directory contents. Supports recursive listing and glob pattern filtering.",
				Parameters: map[string]ParamSpec{
					"path": {
						Type:        "string",
						Description: "Path to the directory to list",
						Required:    true,
					},
					"recursive": {
						Type:        "boolean",
						Description: "List recursively (default: false)",
					},
					"pattern": {
						Type:        "string",
						Description: "Glob pattern to filter entries (e.g. \"*.go\")",
					},
				},
			},
		},
	}
}

type listDirInput struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
	Pattern   string `json:"pattern"`
}

type listDirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "dir"
	Size int64  `json:"size"`
}

type listDirOutput struct {
	Entries []listDirEntry `json:"entries"`
	Total   int            `json:"total"`
}

// Info returns the tool info for Eino registration.
func (t *ListDirTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ListDirManifest().Tools[0]), nil
}

// InvokableRun lists the directory contents.
func (t *ListDirTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input listDirInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("list_dir: parse input: %w", err)
	}
	if input.Path == "" {
		return "", fmt.Errorf("list_dir: path is required")
	}

	var entries []listDirEntry

	if input.Recursive {
		entries = listDirRecursive(input.Path, input.Pattern)
	} else {
		var err error
		entries, err = listDirFlat(input.Path, input.Pattern)
		if err != nil {
			return "", fmt.Errorf("list_dir: %w", err)
		}
	}

	result := listDirOutput{
		Entries: entries,
		Total:   len(entries),
	}
	if result.Entries == nil {
		result.Entries = []listDirEntry{}
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("list_dir: marshal result: %w", err)
	}
	return string(out), nil
}

func listDirFlat(dir, pattern string) ([]listDirEntry, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var entries []listDirEntry
	for _, de := range dirEntries {
		if pattern != "" {
			matched, _ := filepath.Match(pattern, de.Name())
			if !matched {
				continue
			}
		}

		entry := listDirEntry{
			Name: de.Name(),
			Path: filepath.Join(dir, de.Name()),
			Type: entryType(de),
		}
		if info, err := de.Info(); err == nil {
			entry.Size = info.Size()
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func listDirRecursive(root, pattern string) []listDirEntry {
	var entries []listDirEntry
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		// Skip the root directory itself
		if path == root {
			return nil
		}
		if pattern != "" {
			matched, _ := filepath.Match(pattern, d.Name())
			if !matched {
				return nil
			}
		}

		entry := listDirEntry{
			Name: d.Name(),
			Path: path,
			Type: entryType(d),
		}
		if info, err := d.Info(); err == nil {
			entry.Size = info.Size()
		}
		entries = append(entries, entry)

		if len(entries) >= maxListEntries {
			return filepath.SkipAll
		}
		return nil
	})
	return entries
}

func entryType(d fs.DirEntry) string {
	if d.IsDir() {
		return "dir"
	}
	return "file"
}

var _ tool.InvokableTool = (*ListDirTool)(nil)
