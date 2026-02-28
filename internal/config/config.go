package config

import "time"

// Config is the root configuration for Ozzie.
type Config struct {
	Gateway GatewayConfig `json:"gateway"`
	Models  ModelsConfig  `json:"models"`
	Events  EventsConfig  `json:"events"`
	Agent     AgentConfig     `json:"agent"`
	Embedding EmbeddingConfig `json:"embedding"`
	Plugins   PluginsConfig   `json:"plugins"`
	Skills  SkillsConfig  `json:"skills"`
	Tools   ToolsConfig   `json:"tools"`
	Sandbox SandboxConfig `json:"sandbox"`
	Runtime RuntimeConfig `json:"runtime"`
	Web     WebConfig     `json:"web"`
}

// WebConfig configures web search and fetch capabilities.
type WebConfig struct {
	Search WebSearchConfig `json:"search"`
	Fetch  WebFetchConfig  `json:"fetch"`
}

// WebSearchConfig configures the web search tool.
type WebSearchConfig struct {
	Enabled      *bool  `json:"enabled"`                // default: true
	Provider     string `json:"provider"`               // "duckduckgo" (default) | "google" | "bing"
	Timeout      string `json:"timeout,omitempty"`      // default: "30s"
	MaxResults   int    `json:"max_results"`            // default: 10
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
	Timeout   string `json:"timeout,omitempty"`  // default: "30s"
	MaxBodyKB int    `json:"max_body_kb"`        // default: 512
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
	Environment     string `json:"environment,omitempty"`        // "local" | "container"
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
	Dir     string   `json:"dir"`     // plugin directory (default: $OZZIE_PATH/plugins)
	Enabled []string `json:"enabled"` // enabled plugin names (empty = all)
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
	Driver    string         `json:"driver"` // "anthropic", "openai"
	Model     string         `json:"model"`
	BaseURL   string         `json:"base_url,omitempty"`
	Auth      AuthConfig     `json:"auth"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	ContextWindow int            `json:"context_window,omitempty"` // total context window in tokens (0 = driver default)
	MaxConcurrent int            `json:"max_concurrent,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	Tier          string         `json:"tier,omitempty"` // "small" | "medium" | "large" (auto-detected if empty)
	Timeout   Duration       `json:"timeout,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
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

// CoordinatorConfig configures the coordinator pattern defaults.
type CoordinatorConfig struct {
	DefaultLevel        string `json:"default_level"`         // "disabled" | "supervised" | "autonomous" (default: "disabled")
	MaxValidationRounds int    `json:"max_validation_rounds"` // max plan-revise cycles before failure (default: 3)
}

// AgentConfig holds agent settings.
type AgentConfig struct {
	SystemPrompt string            `json:"system_prompt,omitempty"`
	Coordinator  CoordinatorConfig `json:"coordinator"`
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
