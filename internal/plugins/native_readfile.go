package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ReadFileTool reads file contents with optional offset and limit.
type ReadFileTool struct{}

// NewReadFileTool creates a new read_file tool.
func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{}
}

// ReadFileManifest returns the plugin manifest for the read_file tool.
func ReadFileManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "read_file",
		Description: "Read the contents of a file",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Capabilities: CapabilitySet{
			Filesystem: &FSCapability{ReadOnly: true},
		},
		Tools: []ToolSpec{
			{
				Name:        "read_file",
				Description: "Read the contents of a file. Returns the text content with optional line offset and limit.",
				Parameters: map[string]ParamSpec{
					"path": {
						Type:        "string",
						Description: "Path to the file to read",
						Required:    true,
					},
					"offset": {
						Type:        "integer",
						Description: "Line offset (0-based) to start reading from",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of lines to return",
					},
				},
			},
		},
	}
}

type readFileInput struct {
	Path   string `json:"path"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

type readFileOutput struct {
	Content   string `json:"content"`
	Lines     int    `json:"lines"`
	Truncated bool   `json:"truncated"`
}

// Info returns the tool info for Eino registration.
func (t *ReadFileTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ReadFileManifest().Tools[0]), nil
}

// InvokableRun reads the file and returns its contents.
func (t *ReadFileTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input readFileInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("read_file: parse input: %w", err)
	}
	if input.Path == "" {
		return "", fmt.Errorf("read_file: path is required")
	}

	data, err := os.ReadFile(input.Path)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}

	lines := bytes.Split(data, []byte("\n"))
	totalLines := len(lines)
	truncated := false

	if input.Offset > 0 {
		if input.Offset >= len(lines) {
			lines = nil
		} else {
			lines = lines[input.Offset:]
		}
	}

	if input.Limit > 0 && input.Limit < len(lines) {
		lines = lines[:input.Limit]
		truncated = true
	}

	var parts []string
	for _, l := range lines {
		parts = append(parts, string(l))
	}

	result := readFileOutput{
		Content:   strings.Join(parts, "\n"),
		Lines:     totalLines,
		Truncated: truncated,
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("read_file: marshal result: %w", err)
	}
	return string(out), nil
}

var _ tool.InvokableTool = (*ReadFileTool)(nil)
