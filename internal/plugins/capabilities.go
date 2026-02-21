package plugins

import (
	extism "github.com/extism/go-sdk"
)

// CapabilitySet defines what a plugin is allowed to do (deny-by-default).
type CapabilitySet struct {
	HTTP       *HTTPCapability `json:"http,omitempty"`
	KV         bool            `json:"kv"`
	Log        bool            `json:"log"`
	Filesystem *FSCapability   `json:"filesystem,omitempty"`
	Secrets    []string        `json:"secrets,omitempty"`
	Exec       bool            `json:"exec"`
	Elevated   bool            `json:"elevated"`
	Memory     *MemoryLimit    `json:"memory,omitempty"`
	Timeout    int             `json:"timeout,omitempty"` // milliseconds
}

// HTTPCapability allows network access to specific hosts.
type HTTPCapability struct {
	AllowedHosts []string `json:"allowed_hosts"`
}

// FSCapability allows filesystem access to specific paths.
type FSCapability struct {
	AllowedPaths map[string]string `json:"allowed_paths"` // host path → guest path
	ReadOnly     bool              `json:"read_only"`
}

// MemoryLimit constrains WASM memory usage.
type MemoryLimit struct {
	MaxPages uint32 `json:"max_pages"` // 1 page = 64 KiB
}

// BuildExtismManifest converts a PluginManifest into an extism.Manifest
// with deny-by-default capabilities.
func BuildExtismManifest(m *PluginManifest) extism.Manifest {
	em := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{Path: m.WasmPath},
		},
		Config: m.Config,
	}

	caps := m.Capabilities

	// HTTP: deny-by-default — only allow if explicitly listed
	if caps.HTTP != nil && len(caps.HTTP.AllowedHosts) > 0 {
		em.AllowedHosts = caps.HTTP.AllowedHosts
	}

	// Filesystem: deny-by-default
	if caps.Filesystem != nil && len(caps.Filesystem.AllowedPaths) > 0 {
		em.AllowedPaths = caps.Filesystem.AllowedPaths
	}

	// Memory limits
	if caps.Memory != nil && caps.Memory.MaxPages > 0 {
		em.Memory = &extism.ManifestMemory{
			MaxPages: caps.Memory.MaxPages,
		}
	}

	// Timeout
	if caps.Timeout > 0 {
		em.Timeout = uint64(caps.Timeout)
	}

	return em
}
