// Package plugins provides the Ozzie plugin system.
package plugins

import (
	"fmt"
	"os"

	"github.com/marcozac/go-jsonc"
)

// PluginManifest describes a plugin's metadata, capabilities, and tools.
type PluginManifest struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Level        string            `json:"level"`     // "tool" or "communication"
	Provider     string            `json:"provider"`  // "extism" or "native"
	WasmPath     string            `json:"wasm_path"` // path to .wasm file (extism only)
	Dangerous    bool              `json:"dangerous"` // default for all tools
	Capabilities CapabilitySet     `json:"capabilities"`
	Tools        []ToolSpec        `json:"tools"` // 1..N tools per plugin
	Config       map[string]string `json:"config"`
}

// ToolSpec describes a single tool interface exposed by a plugin.
type ToolSpec struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Parameters  map[string]ParamSpec `json:"parameters"`
	Func        string               `json:"func,omitempty"` // WASM export name (default: "handle")
	Dangerous   bool                 `json:"dangerous"`      // per-tool override
}

// ParamSpec describes a single tool parameter.
type ParamSpec struct {
	Type        string               `json:"type"` // "string", "number", "boolean", "integer", "array", "object"
	Description string               `json:"description"`
	Required    bool                 `json:"required"`
	Enum        []string             `json:"enum,omitempty"`
	Default     any                  `json:"default,omitempty"`
	Items       *ParamSpec           `json:"items,omitempty"`      // element schema for arrays
	Properties  map[string]ParamSpec `json:"properties,omitempty"` // sub-properties for objects
}

// LoadManifest reads and parses a JSONC manifest file.
func LoadManifest(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}

	var m PluginManifest
	if err := jsonc.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}

	if m.Name == "" {
		return nil, fmt.Errorf("manifest %s: name is required", path)
	}
	if len(m.Tools) == 0 {
		return nil, fmt.Errorf("manifest %s: at least one tool is required", path)
	}

	for i := range m.Tools {
		// Default Func to "handle"
		if m.Tools[i].Func == "" {
			m.Tools[i].Func = "handle"
		}
		// Default Name to manifest name (only if single tool)
		if m.Tools[i].Name == "" {
			if len(m.Tools) == 1 {
				m.Tools[i].Name = m.Name
			} else {
				return nil, fmt.Errorf("manifest %s: tool at index %d must have a name", path, i)
			}
		}
		// Propagate manifest-level dangerous flag
		if m.Dangerous {
			m.Tools[i].Dangerous = true
		}
	}

	return &m, nil
}
