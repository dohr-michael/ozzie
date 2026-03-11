package config

import (
	"slices"
	"time"
)

// Config is the root configuration for Ozzie.
type Config struct {
	Gateway        GatewayConfig        `json:"gateway"`
	Models         ModelsConfig         `json:"models"`
	Events         EventsConfig         `json:"events"`
	Agent          AgentConfig          `json:"agent"`
	Embedding      EmbeddingConfig      `json:"embedding"`
	Plugins        PluginsConfig        `json:"plugins"`
	Skills         SkillsConfig         `json:"skills"`
	Tools          ToolsConfig          `json:"tools"`
	Sandbox        SandboxConfig        `json:"sandbox"`
	Runtime        RuntimeConfig        `json:"runtime"`
	Web            WebConfig            `json:"web"`
	MCP            MCPConfig            `json:"mcp"`
	LayeredContext LayeredContextConfig `json:"layered_context"`
	Policies       PoliciesConfig       `json:"policies"`
	Connectors     ConnectorsConfig     `json:"connectors"`
}

// ConnectorsConfig configures external platform connectors.
type ConnectorsConfig struct {
	Discord *DiscordConnectorConfig `json:"discord,omitempty"`
}

// DiscordConnectorConfig configures the Discord connector.
type DiscordConnectorConfig struct {
	Token        string `json:"token"`                   // bot token (supports ${{ .Env.DISCORD_TOKEN }})
	AdminChannel string `json:"admin_channel,omitempty"` // channel ID for admin notifications
}

// PoliciesConfig configures policy overrides for the predefined policies.
type PoliciesConfig struct {
	Overrides map[string]PolicyOverride `json:"overrides,omitempty"`
}

// PolicyOverride allows customizing a predefined policy via config.
// Zero values are ignored (the default is kept).
type PolicyOverride struct {
	AllowedSkills []string `json:"allowed_skills,omitempty"`
	AllowedTools  []string `json:"allowed_tools,omitempty"`
	DeniedTools   []string `json:"denied_tools,omitempty"`
	ApprovalMode  string   `json:"approval_mode,omitempty"`
	ClientFacing  *bool    `json:"client_facing,omitempty"`
	MaxConcurrent int      `json:"max_concurrent,omitempty"`
}

// MCPConfig configures external MCP server connections.
type MCPConfig struct {
	Servers map[string]*MCPServerConfig `json:"servers,omitempty"`
}

// MCPServerConfig configures a single external MCP server.
type MCPServerConfig struct {
	Transport    string            `json:"transport"`               // "stdio" | "sse" | "http"
	Command      string            `json:"command,omitempty"`       // stdio: command to launch
	Args         []string          `json:"args,omitempty"`          // stdio: command arguments
	Env          map[string]string `json:"env,omitempty"`           // env vars passed to subprocess (supports ${{ .Env.VAR }})
	URL          string            `json:"url,omitempty"`           // sse/http: endpoint URL
	Dangerous    *bool             `json:"dangerous,omitempty"`     // default: true — all tools from this server
	AllowedTools []string          `json:"allowed_tools,omitempty"` // empty = all tools allowed
	DeniedTools  []string          `json:"denied_tools,omitempty"`  // blacklist (takes priority over allowed)
	TrustedTools []string          `json:"trusted_tools,omitempty"` // tools NOT marked dangerous (bypass confirmation)
	Timeout      int               `json:"timeout,omitempty"`       // ms per CallTool (default: 30000)
}

// IsDangerous returns true if the server's tools should be marked dangerous (default: true).
func (c *MCPServerConfig) IsDangerous() bool {
	if c.Dangerous == nil {
		return true
	}
	return *c.Dangerous
}

// LayeredContextConfig configures the layered context compression system.
type LayeredContextConfig struct {
	Enabled           *bool `json:"enabled"`             // default: false
	MaxArchives       int   `json:"max_archives"`        // default: 12
	MaxRecentMessages int   `json:"max_recent_messages"` // default: 24
	ArchiveChunkSize  int   `json:"archive_chunk_size"`  // default: 8
}

// IsEnabled returns true if layered context is enabled (default: false).
func (c LayeredContextConfig) IsEnabled() bool {
	return c.Enabled != nil && *c.Enabled
}

// WebConfig configures web search and fetch capabilities.
type WebConfig struct {
	Search WebSearchConfig `json:"search"`
	Fetch  WebFetchConfig  `json:"fetch"`
}

// WebSearchConfig configures the web search tool.
type WebSearchConfig struct {
	Enabled      *bool  `json:"enabled"`           // default: true
	Provider     string `json:"provider"`          // "duckduckgo" (default) | "google" | "bing"
	Timeout      string `json:"timeout,omitempty"` // default: "30s"
	MaxResults   int    `json:"max_results"`       // default: 10
	GoogleAPIKey string `json:"google_api_key,omitempty"`
	GoogleCX     string `json:"google_cx,omitempty"`
	BingAPIKey   string `json:"bing_api_key,omitempty"`
}

// IsSearchEnabled returns true if web search is enabled (default: true).
func (c WebSearchConfig) IsSearchEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// WebFetchConfig configures the web fetch tool.
type WebFetchConfig struct {
	Enabled   *bool  `json:"enabled"`           // default: true
	Timeout   string `json:"timeout,omitempty"` // default: "30s"
	MaxBodyKB int    `json:"max_body_kb"`       // default: 512
	UserAgent string `json:"user_agent,omitempty"`
}

// IsFetchEnabled returns true if web fetch is enabled (default: true).
func (c WebFetchConfig) IsFetchEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// RuntimeConfig configures the runtime environment awareness.
type RuntimeConfig struct {
	Environment     string `json:"environment,omitempty"`       // "local" | "container"
	SystemToolsFile string `json:"system_tools_file,omitempty"` // path to auto-generated tools JSON
}

// SandboxConfig configures the sandbox guard for autonomous sub-agents.
type SandboxConfig struct {
	Enabled      *bool    `json:"enabled"`       // default: true
	AllowedPaths []string `json:"allowed_paths"` // extra paths allowed outside WorkDir
}

// IsSandboxEnabled returns true if the sandbox is enabled (default: true).
func (c SandboxConfig) IsSandboxEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// EmbeddingConfig configures the embedding model for semantic memory.
type EmbeddingConfig struct {
	Enabled   *bool      `json:"enabled"`              // default: false (opt-in)
	Driver    string     `json:"driver"`               // "openai" | "ollama"
	Model     string     `json:"model"`                // e.g. "text-embedding-3-small", "nomic-embed-text"
	BaseURL   string     `json:"base_url,omitempty"`   // for ollama or custom endpoints
	Dims      int        `json:"dims,omitempty"`       // embedding dimensions (OpenAI v3 supports this)
	Auth      AuthConfig `json:"auth,omitempty"`       // reuses existing AuthConfig
	QueueSize int        `json:"queue_size,omitempty"` // buffer channel size (default: 100)
}

// IsEnabled returns true if embeddings are enabled (default: false).
func (c EmbeddingConfig) IsEnabled() bool {
	return c.Enabled != nil && *c.Enabled
}

// ToolsConfig configures tool permissions.
type ToolsConfig struct {
	AllowedDangerous []string `json:"allowed_dangerous"` // globally auto-approved dangerous tools
}

// SkillsConfig configures the skill system.
type SkillsConfig struct {
	Dirs    []string `json:"dirs"`    // skill directories (default: [$OZZIE_PATH/skills])
	Enabled []string `json:"enabled"` // enabled skill names (empty = all)
}

// PluginsConfig configures the plugin system.
type PluginsConfig struct {
	Dir            string                                `json:"dir"`     // plugin directory (default: $OZZIE_PATH/plugins)
	Enabled        []string                              `json:"enabled"` // enabled plugin names (empty = all)
	Authorizations map[string]*PluginAuthorizationConfig `json:"authorizations,omitempty"`
}

// PluginAuthorizationConfig defines what Ozzie authorizes for a specific plugin.
// Mirror types live here to avoid import cycles (config -> plugins).
type PluginAuthorizationConfig struct {
	HTTP       *HTTPAuthConfig       `json:"http,omitempty"`
	Filesystem *FSAuthConfig         `json:"filesystem,omitempty"`
	Secrets    *SecretsAuthConfig    `json:"secrets,omitempty"`
	Deny       []string              `json:"deny,omitempty"`
	Resources  *ResourceLimitsConfig `json:"resources,omitempty"`
}

// HTTPAuthConfig authorizes HTTP access to specific hosts.
type HTTPAuthConfig struct {
	AllowedHosts []string `json:"allowed_hosts"`
}

// FSAuthConfig authorizes filesystem access to specific paths.
type FSAuthConfig struct {
	AllowedPaths map[string]string `json:"allowed_paths"`
	ReadOnly     bool              `json:"read_only,omitempty"`
}

// SecretsAuthConfig authorizes access to specific secrets.
type SecretsAuthConfig struct {
	Allowed []string `json:"allowed"`
}

// ResourceLimitsConfig configures resource limits for a plugin.
type ResourceLimitsConfig struct {
	MemoryMaxPages uint32 `json:"memory_max_pages,omitempty"`
	Timeout        int    `json:"timeout,omitempty"` // milliseconds
}

// GatewayConfig holds the gateway server settings.
type GatewayConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// ModelsConfig holds model provider configuration.
type ModelsConfig struct {
	Default   string                    `json:"default"`
	Providers map[string]ProviderConfig `json:"providers"`
}

// ProviderConfig configures a single LLM provider.
type ProviderConfig struct {
	Driver        string         `json:"driver"` // "anthropic", "openai"
	Model         string         `json:"model"`
	BaseURL       string         `json:"base_url,omitempty"`
	Auth          AuthConfig     `json:"auth"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	ContextWindow int            `json:"context_window,omitempty"` // total context window in tokens (0 = driver default)
	MaxConcurrent int            `json:"max_concurrent,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	Capabilities  []string       `json:"capabilities,omitempty"`  // e.g. ["thinking", "tool_use", "coding"]
	PromptPrefix  string         `json:"prompt_prefix,omitempty"` // custom instruction injected for this overlay
	Tier          string         `json:"tier,omitempty"`          // "small" | "medium" | "large" (auto-detected if empty)
	Timeout       Duration       `json:"timeout,omitempty"`
	Options       map[string]any `json:"options,omitempty"`
	Retry         *RetryConfig   `json:"retry,omitempty"`    // retry + circuit breaker config
	Fallback      string         `json:"fallback,omitempty"` // name of fallback provider
}

// Equal returns true if two ProviderConfigs are equivalent (field-by-field comparison).
func (p ProviderConfig) Equal(other ProviderConfig) bool {
	if p.Driver != other.Driver || p.Model != other.Model || p.BaseURL != other.BaseURL {
		return false
	}
	if p.Auth != other.Auth || p.MaxTokens != other.MaxTokens || p.ContextWindow != other.ContextWindow {
		return false
	}
	if p.MaxConcurrent != other.MaxConcurrent || p.PromptPrefix != other.PromptPrefix {
		return false
	}
	if p.Tier != other.Tier || p.Timeout != other.Timeout || p.Fallback != other.Fallback {
		return false
	}
	if !slices.Equal(p.Tags, other.Tags) || !slices.Equal(p.Capabilities, other.Capabilities) {
		return false
	}
	return true
}

// RetryConfig configures retry behavior with exponential backoff.
type RetryConfig struct {
	MaxAttempts  int     `json:"max_attempts,omitempty"`  // total attempts including first (default: 3)
	InitialDelay Duration `json:"initial_delay,omitempty"` // base delay before first retry (default: 1s)
	MaxDelay     Duration `json:"max_delay,omitempty"`     // delay cap (default: 30s)
	Multiplier   float64 `json:"multiplier,omitempty"`    // backoff multiplier (default: 2.0)
}

// AuthConfig configures API key resolution.
type AuthConfig struct {
	APIKey string `json:"api_key,omitempty"` // Direct API key or ${{ .Env.VAR }} template
	Token  string `json:"token,omitempty"`   // OAuth/Bearer token (e.g. Claude Code token)
}

// EventsConfig holds event bus settings.
type EventsConfig struct {
	BufferSize int    `json:"buffer_size"`
	LogLevel   string `json:"log_level"` // "debug" | "info" | "warn" | "error" (default: "info")
}

// AgentConfig holds agent settings.
type AgentConfig struct {
	SystemPrompt      string `json:"system_prompt,omitempty"`
	PreferredLanguage string `json:"preferred_language,omitempty"` // e.g. "en", "fr"
}

// Duration wraps time.Duration for JSON unmarshaling.
type Duration time.Duration

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	// Remove quotes
	s := string(b)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Duration(d).String() + `"`), nil
}
