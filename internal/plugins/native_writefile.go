package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// WriteFileTool writes content to a file.
type WriteFileTool struct{}

// NewWriteFileTool creates a new write_file tool.
func NewWriteFileTool() *WriteFileTool {
	return &WriteFileTool{}
}

// WriteFileManifest returns the plugin manifest for the write_file tool.
func WriteFileManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "write_file",
		Description: "Write content to a file",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   true,
		Capabilities: CapabilitySet{
			Filesystem: &FSCapability{},
		},
		Tools: []ToolSpec{
			{
				Name:        "write_file",
				Description: "Write content to a file. Creates parent directories by default. Returns the absolute path and bytes written.",
				Parameters: map[string]ParamSpec{
					"path": {
						Type:        "string",
						Description: "Path to the file to write",
						Required:    true,
					},
					"content": {
						Type:        "string",
						Description: "Content to write to the file",
						Required:    true,
					},
					"create_dirs": {
						Type:        "boolean",
						Description: "Create parent directories if they don't exist (default: true)",
					},
				},
				Dangerous: true,
			},
		},
	}
}

type writeFileInput struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	CreateDirs *bool  `json:"create_dirs"`
}

type writeFileOutput struct {
	Path         string `json:"path"`
	BytesWritten int    `json:"bytes_written"`
}

// Info returns the tool info for Eino registration.
func (t *WriteFileTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&WriteFileManifest().Tools[0]), nil
}

// InvokableRun writes content to the file.
func (t *WriteFileTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input writeFileInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("write_file: parse input: %w", err)
	}
	if input.Path == "" {
		return "", fmt.Errorf("write_file: path is required")
	}

	createDirs := true
	if input.CreateDirs != nil {
		createDirs = *input.CreateDirs
	}

	absPath, err := filepath.Abs(input.Path)
	if err != nil {
		return "", fmt.Errorf("write_file: resolve path: %w", err)
	}

	if createDirs {
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("write_file: create dirs: %w", err)
		}
	}

	data := []byte(input.Content)
	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return "", fmt.Errorf("write_file: %w", err)
	}

	result := writeFileOutput{
		Path:         absPath,
		BytesWritten: len(data),
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("write_file: marshal result: %w", err)
	}
	return string(out), nil
}

var _ tool.InvokableTool = (*WriteFileTool)(nil)
