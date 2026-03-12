package hands

import (
	"encoding/json"
	"fmt"

	extism "github.com/extism/go-sdk"

	"github.com/dohr-michael/ozzie/internal/config"
)

// ---------------------------------------------------------------------------
// Plugin Capabilities — what a plugin declares it needs (developer-side)
// ---------------------------------------------------------------------------

// PluginCapabilities declares the capabilities a plugin needs.
// This is the developer's declaration in the manifest — it does NOT grant access.
// Access is granted by PluginAuthorization (user-side config).
type PluginCapabilities struct {
	HTTP       bool                `json:"http,omitempty"`
	KV         bool                `json:"kv,omitempty"`
	Log        bool                `json:"log,omitempty"`
	Filesystem *FSCapabilityIntent `json:"filesystem,omitempty"` // nil = not needed
	Secrets    []string            `json:"secrets,omitempty"`
	Exec       bool                `json:"exec,omitempty"`
	Elevated   bool                `json:"elevated,omitempty"`
}

// FSCapabilityIntent declares a plugin's filesystem needs.
type FSCapabilityIntent struct {
	ReadOnly bool `json:"read_only,omitempty"` // intrinsic constraint of the plugin
}

// UnmarshalJSON supports both `"filesystem": true` (shorthand for read-write)
// and `"filesystem": {"read_only": true}` (explicit).
func (c *PluginCapabilities) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion.
	type alias struct {
		HTTP       bool            `json:"http,omitempty"`
		KV         bool            `json:"kv,omitempty"`
		Log        bool            `json:"log,omitempty"`
		Filesystem json.RawMessage `json:"filesystem,omitempty"`
		Secrets    []string        `json:"secrets,omitempty"`
		Exec       bool            `json:"exec,omitempty"`
		Elevated   bool            `json:"elevated,omitempty"`
	}

	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	c.HTTP = a.HTTP
	c.KV = a.KV
	c.Log = a.Log
	c.Secrets = a.Secrets
	c.Exec = a.Exec
	c.Elevated = a.Elevated

	if len(a.Filesystem) > 0 {
		// Try bool first
		var boolVal bool
		if err := json.Unmarshal(a.Filesystem, &boolVal); err == nil {
			if boolVal {
				c.Filesystem = &FSCapabilityIntent{ReadOnly: false}
			}
			return nil
		}
		// Try object
		var intent FSCapabilityIntent
		if err := json.Unmarshal(a.Filesystem, &intent); err != nil {
			return fmt.Errorf("capabilities.filesystem: expected bool or object: %w", err)
		}
		c.Filesystem = &intent
	}

	return nil
}

// ---------------------------------------------------------------------------
// Plugin Authorization — what Ozzie allows (user-side config)
// ---------------------------------------------------------------------------

// PluginAuthorization defines what Ozzie authorizes for a specific plugin.
type PluginAuthorization struct {
	HTTP       *HTTPAuth       `json:"http,omitempty"`
	Filesystem *FSAuth         `json:"filesystem,omitempty"`
	Secrets    *SecretsAuth    `json:"secrets,omitempty"`
	Deny       []string        `json:"deny,omitempty"` // capabilities to deny (e.g. "exec", "kv")
	Resources  *ResourceLimits `json:"resources,omitempty"`
}

// HTTPAuth authorizes HTTP access to specific hosts.
type HTTPAuth struct {
	AllowedHosts []string `json:"allowed_hosts"`
}

// FSAuth authorizes filesystem access to specific paths.
type FSAuth struct {
	AllowedPaths map[string]string `json:"allowed_paths"` // host path -> guest path
	ReadOnly     bool              `json:"read_only,omitempty"`
}

// SecretsAuth authorizes access to specific secrets.
type SecretsAuth struct {
	Allowed []string `json:"allowed"`
}

// ---------------------------------------------------------------------------
// Resource Limits — non-security, stays in manifest
// ---------------------------------------------------------------------------

// ResourceLimits constrains plugin resource usage.
type ResourceLimits struct {
	Memory  *MemoryLimit `json:"memory,omitempty"`
	Timeout int          `json:"timeout,omitempty"` // milliseconds
}

// MemoryLimit constrains WASM memory usage.
type MemoryLimit struct {
	MaxPages uint32 `json:"max_pages"` // 1 page = 64 KiB
}

// ---------------------------------------------------------------------------
// Resolved Capabilities — the merged result
// ---------------------------------------------------------------------------

// ResolvedCapabilities is the result of merging PluginCapabilities with
// PluginAuthorization. This is what BuildExtismManifest and sandbox use.
type ResolvedCapabilities struct {
	HTTP       *HTTPAuth
	KV         bool
	Log        bool
	Filesystem *ResolvedFS
	Secrets    []string
	Exec       bool
	Elevated   bool
	Resources  ResourceLimits
}

// ResolvedFS is the merged filesystem capability.
type ResolvedFS struct {
	AllowedPaths map[string]string
	ReadOnly     bool
}

// ResolveCapabilities merges a plugin's declared capabilities with the user's
// authorization config. Deny-by-default: capabilities requested but not
// authorized get empty/false values.
func ResolveCapabilities(caps PluginCapabilities, auth *PluginAuthorization, limits ResourceLimits) ResolvedCapabilities {
	resolved := ResolvedCapabilities{
		KV:        caps.KV,
		Log:       caps.Log,
		Exec:      caps.Exec,
		Elevated:  caps.Elevated,
		Resources: limits,
	}

	// HTTP: deny-by-default — need both capability AND authorization
	if caps.HTTP && auth != nil && auth.HTTP != nil {
		resolved.HTTP = auth.HTTP
	}

	// Filesystem: deny-by-default
	if caps.Filesystem != nil {
		resolved.Filesystem = &ResolvedFS{
			// Plugin says read_only → cannot be upgraded to read-write by auth
			ReadOnly: caps.Filesystem.ReadOnly,
		}
		if auth != nil && auth.Filesystem != nil {
			resolved.Filesystem.AllowedPaths = auth.Filesystem.AllowedPaths
			// Auth can further restrict to read-only, but cannot upgrade
			if auth.Filesystem.ReadOnly {
				resolved.Filesystem.ReadOnly = true
			}
		}
	}

	// Secrets: intersection of requested and authorized
	if len(caps.Secrets) > 0 && auth != nil && auth.Secrets != nil {
		allowedSet := make(map[string]bool, len(auth.Secrets.Allowed))
		for _, s := range auth.Secrets.Allowed {
			allowedSet[s] = true
		}
		for _, s := range caps.Secrets {
			if allowedSet[s] {
				resolved.Secrets = append(resolved.Secrets, s)
			}
		}
	}

	// Deny overrides: disable binary capabilities
	if auth != nil {
		denySet := make(map[string]bool, len(auth.Deny))
		for _, d := range auth.Deny {
			denySet[d] = true
		}
		if denySet["exec"] {
			resolved.Exec = false
		}
		if denySet["elevated"] {
			resolved.Elevated = false
		}
		if denySet["kv"] {
			resolved.KV = false
		}
		if denySet["log"] {
			resolved.Log = false
		}
		if denySet["http"] {
			resolved.HTTP = nil
		}
		if denySet["filesystem"] {
			resolved.Filesystem = nil
		}
	}

	// Override resources from auth if provided
	if auth != nil && auth.Resources != nil {
		if auth.Resources.Memory != nil {
			resolved.Resources.Memory = auth.Resources.Memory
		}
		if auth.Resources.Timeout > 0 {
			resolved.Resources.Timeout = auth.Resources.Timeout
		}
	}

	return resolved
}

// ValidateAuthorization checks if an authorization references capabilities that
// the plugin did not request. Returns warnings (not errors).
func ValidateAuthorization(pluginName string, caps PluginCapabilities, auth *PluginAuthorization) []string {
	if auth == nil {
		return nil
	}
	var warnings []string

	if auth.HTTP != nil && !caps.HTTP {
		warnings = append(warnings, fmt.Sprintf("plugin %q: authorization grants HTTP but plugin does not request it", pluginName))
	}
	if auth.Filesystem != nil && caps.Filesystem == nil {
		warnings = append(warnings, fmt.Sprintf("plugin %q: authorization grants filesystem but plugin does not request it", pluginName))
	}
	if auth.Secrets != nil && len(caps.Secrets) == 0 {
		warnings = append(warnings, fmt.Sprintf("plugin %q: authorization grants secrets but plugin does not request any", pluginName))
	}

	return warnings
}

// ---------------------------------------------------------------------------
// Config conversion
// ---------------------------------------------------------------------------

// AuthFromConfig converts a config-level PluginAuthorizationConfig into a
// PluginAuthorization. This bridges the config and plugins packages.
func AuthFromConfig(cfg *config.PluginAuthorizationConfig) *PluginAuthorization {
	if cfg == nil {
		return nil
	}
	auth := &PluginAuthorization{
		Deny: cfg.Deny,
	}
	if cfg.HTTP != nil {
		auth.HTTP = &HTTPAuth{AllowedHosts: cfg.HTTP.AllowedHosts}
	}
	if cfg.Filesystem != nil {
		auth.Filesystem = &FSAuth{
			AllowedPaths: cfg.Filesystem.AllowedPaths,
			ReadOnly:     cfg.Filesystem.ReadOnly,
		}
	}
	if cfg.Secrets != nil {
		auth.Secrets = &SecretsAuth{Allowed: cfg.Secrets.Allowed}
	}
	if cfg.Resources != nil {
		auth.Resources = &ResourceLimits{
			Timeout: cfg.Resources.Timeout,
		}
		if cfg.Resources.MemoryMaxPages > 0 {
			auth.Resources.Memory = &MemoryLimit{MaxPages: cfg.Resources.MemoryMaxPages}
		}
	}
	return auth
}

// ---------------------------------------------------------------------------
// Extism manifest builder (uses Resolved)
// ---------------------------------------------------------------------------

// BuildExtismManifest converts a PluginManifest into an extism.Manifest
// using the resolved capabilities (post-authorization merge).
func BuildExtismManifest(m *PluginManifest) extism.Manifest {
	em := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{Path: m.WasmPath},
		},
		Config: m.Config,
	}

	resolved := m.Resolved
	if resolved == nil {
		return em
	}

	// HTTP: use resolved allowed hosts
	if resolved.HTTP != nil && len(resolved.HTTP.AllowedHosts) > 0 {
		em.AllowedHosts = resolved.HTTP.AllowedHosts
	}

	// Filesystem: use resolved allowed paths
	if resolved.Filesystem != nil && len(resolved.Filesystem.AllowedPaths) > 0 {
		em.AllowedPaths = resolved.Filesystem.AllowedPaths
	}

	// Memory limits
	if resolved.Resources.Memory != nil && resolved.Resources.Memory.MaxPages > 0 {
		em.Memory = &extism.ManifestMemory{
			MaxPages: resolved.Resources.Memory.MaxPages,
		}
	}

	// Timeout
	if resolved.Resources.Timeout > 0 {
		em.Timeout = uint64(resolved.Resources.Timeout)
	}

	return em
}
