package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const defaultMaxResults = 50

// skipDirs are directories to skip during search.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".hg":          true,
}

// SearchTool searches file contents using regex patterns.
type SearchTool struct{}

// NewSearchTool creates a new search tool.
func NewSearchTool() *SearchTool {
	return &SearchTool{}
}

// SearchManifest returns the plugin manifest for the search tool.
func SearchManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "search",
		Description: "Search file contents using regex patterns",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Capabilities: CapabilitySet{
			Filesystem: &FSCapability{ReadOnly: true},
		},
		Tools: []ToolSpec{
			{
				Name:        "search",
				Description: "Search file contents using regex patterns. Walks the directory tree, skipping .git/node_modules/vendor/.hg and binary files.",
				Parameters: map[string]ParamSpec{
					"pattern": {
						Type:        "string",
						Description: "Regex pattern to search for",
						Required:    true,
					},
					"path": {
						Type:        "string",
						Description: "Directory to search in (default: current directory)",
					},
					"glob": {
						Type:        "string",
						Description: "Glob pattern to filter files (e.g. \"*.go\")",
					},
					"max_results": {
						Type:        "integer",
						Description: "Maximum number of matches to return (default: 50)",
					},
				},
			},
		},
	}
}

type searchInput struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path"`
	Glob       string `json:"glob"`
	MaxResults int    `json:"max_results"`
}

type searchMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type searchOutput struct {
	Matches   []searchMatch `json:"matches"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated"`
}

// Info returns the tool info for Eino registration.
func (t *SearchTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&SearchManifest().Tools[0]), nil
}

// InvokableRun searches files matching the pattern.
func (t *SearchTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input searchInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("search: parse input: %w", err)
	}
	if input.Pattern == "" {
		return "", fmt.Errorf("search: pattern is required")
	}

	re, err := regexp.Compile(input.Pattern)
	if err != nil {
		return "", fmt.Errorf("search: invalid regex: %w", err)
	}

	searchPath := input.Path
	if searchPath == "" {
		searchPath = "."
	}

	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	var matches []searchMatch
	total := 0
	truncated := false

	err = filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		// Apply glob filter
		if input.Glob != "" {
			matched, _ := filepath.Match(input.Glob, d.Name())
			if !matched {
				return nil
			}
		}

		// Check for binary file
		if isBinary(path) {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil // skip unreadable files
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				total++
				if len(matches) < maxResults {
					matches = append(matches, searchMatch{
						File:    path,
						Line:    lineNum,
						Content: line,
					})
				} else {
					truncated = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("search: walk: %w", err)
	}

	result := searchOutput{
		Matches:   matches,
		Total:     total,
		Truncated: truncated,
	}
	if result.Matches == nil {
		result.Matches = []searchMatch{}
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("search: marshal result: %w", err)
	}
	return string(out), nil
}

// isBinary checks if a file appears to be binary by looking for null bytes
// in the first 512 bytes.
func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return false
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}

var _ tool.InvokableTool = (*SearchTool)(nil)
