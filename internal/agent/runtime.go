package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// SystemTool describes a tool available in the runtime environment.
type SystemTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// LoadSystemTools reads the system tools inventory from a JSON file.
// Returns nil if the file is missing or contains invalid JSON.
func LoadSystemTools(path string) []SystemTool {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("failed to read system tools file", "path", path, "error", err)
		}
		return nil
	}

	var tools []SystemTool
	if err := json.Unmarshal(data, &tools); err != nil {
		slog.Warn("failed to parse system tools file", "path", path, "error", err)
		return nil
	}
	return tools
}

// BuildRuntimeInstruction builds the "## Runtime Environment" prompt section.
// Returns "" in local mode with no tools available.
func BuildRuntimeInstruction(environment string, tools []SystemTool) string {
	if environment == "local" && len(tools) == 0 {
		return ""
	}

	var sb strings.Builder

	if environment == "container" {
		sb.WriteString("## Runtime Environment\n\n")
		sb.WriteString("You are running in **container** mode.\n")
		sb.WriteString("- Tasks involving build, dev, or testing should be isolated in dedicated containers when possible.\n")
		sb.WriteString("- Use `docker run` to launch ephemeral containers for language-specific tasks (e.g., node, python, go).\n")
		sb.WriteString("- You have access to the host Docker daemon via mounted socket.\n")
	}

	if len(tools) > 0 {
		if environment != "container" {
			// Local mode with tools — just the tools section
			sb.WriteString("## System Tools Available\n\n")
		} else {
			sb.WriteString("\n### System Tools Available\n")
		}
		for _, t := range tools {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", t.Name, t.Version))
		}
	}

	return sb.String()
}
