package config

import "time"

// Config is the root configuration for Ozzie.
type Config struct {
	Gateway GatewayConfig `json:"gateway"`
	Models  ModelsConfig  `json:"models"`
	Events  EventsConfig  `json:"events"`
	Agent   AgentConfig   `json:"agent"`
	Plugins PluginsConfig `json:"plugins"`
	Skills  SkillsConfig  `json:"skills"`
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
	MaxTokens int            `json:"max_tokens,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
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
	BufferSize int `json:"buffer_size"`
}

// AgentConfig holds agent settings.
type AgentConfig struct {
	SystemPrompt string `json:"system_prompt,omitempty"`
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
