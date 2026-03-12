package prompt

import (
	"encoding/json"
	"log/slog"
	"os"
)

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
