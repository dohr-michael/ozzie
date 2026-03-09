package setup_wizard

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tailscale/hujson"
)

// ConfigData holds the data for rendering the JSONC config.
type ConfigData struct {
	DefaultProvider   string
	PreferredLanguage string
	Providers         []ProviderConfigData
	Embedding         *EmbeddingConfigData
	LayeredContext    *LayeredContextConfigData
	MCPServers        []MCPServerConfigData
	GatewayHost       string
	GatewayPort       int
}

// MCPServerConfigData holds per-MCP-server config for rendering.
type MCPServerConfigData struct {
	Name         string
	Transport    string
	Command      string
	Args         []string
	URL          string
	Env          map[string]string // name → "${{ .Env.NAME }}" or raw value
	TrustedTools []string
}

// LayeredContextConfigData holds layered context config for rendering.
type LayeredContextConfigData struct {
	MaxRecentMessages int
	MaxArchives       int
}

// EmbeddingConfigData holds embedding config for rendering.
type EmbeddingConfigData struct {
	Driver     string
	Model      string
	BaseURL    string
	AuthEnvVar string
	Dims       int
}

// ProviderConfigData holds per-provider config for rendering.
type ProviderConfigData struct {
	Alias        string
	Driver       string
	Model        string
	BaseURL      string
	AuthEnvVar   string
	Capabilities []string
	Tags         []string
	SystemPrompt string
}

// driverProviderNames maps driver names to friendly provider names.
var driverProviderNames = map[string]string{
	"anthropic":   "claude",
	"openai":      "gpt",
	"openai-like": "custom",
	"gemini":      "gemini",
	"mistral":     "mistral",
	"ollama":      "local",
}

// driverEnvVars maps driver names to their expected env var for the API key.
var driverEnvVars = map[string]string{
	"anthropic": "ANTHROPIC_API_KEY",
	"openai":    "OPENAI_API_KEY",
	"gemini":    "GOOGLE_API_KEY",
	"mistral":   "MISTRAL_API_KEY",
}

// BuildConfigData creates a ConfigData from wizard answers.
func BuildConfigData(answers Answers) ConfigData {
	providers := answers.Providers()
	defaultProvider := answers.String("default_provider", "")

	pdata := make([]ProviderConfigData, len(providers))
	for i, p := range providers {
		pdata[i] = ProviderConfigData{
			Alias:        p.Alias,
			Driver:       p.Driver,
			Model:        p.Model,
			BaseURL:      p.BaseURL,
			AuthEnvVar:   p.EnvVarName,
			Capabilities: p.Capabilities,
			Tags:         p.Tags,
			SystemPrompt: p.SystemPrompt,
		}
	}

	if defaultProvider == "" && len(pdata) > 0 {
		defaultProvider = pdata[0].Alias
	}

	var embData *EmbeddingConfigData
	if emb := answers.Embedding(); emb != nil && emb.Enabled {
		embData = &EmbeddingConfigData{
			Driver:     emb.Driver,
			Model:      emb.Model,
			BaseURL:    emb.BaseURL,
			AuthEnvVar: emb.EnvVarName,
			Dims:       emb.Dims,
		}
	}

	var lcData *LayeredContextConfigData
	if lc := answers.LayeredContext(); lc != nil && lc.Enabled {
		lcData = &LayeredContextConfigData{
			MaxRecentMessages: lc.MaxRecentMessages,
			MaxArchives:       lc.MaxArchives,
		}
	}

	var mcpData []MCPServerConfigData
	for _, srv := range answers.MCPServers() {
		envMap := make(map[string]string, len(srv.EnvVars))
		for _, ev := range srv.EnvVars {
			if ev.IsSecret && ev.Value != "" {
				envMap[ev.Name] = fmt.Sprintf("${{ .Env.%s }}", ev.Name)
			} else if ev.Value != "" {
				envMap[ev.Name] = ev.Value
			} else {
				envMap[ev.Name] = fmt.Sprintf("${{ .Env.%s }}", ev.Name)
			}
		}
		mcpData = append(mcpData, MCPServerConfigData{
			Name:         srv.Name,
			Transport:    srv.Transport,
			Command:      srv.Command,
			Args:         srv.Args,
			URL:          srv.URL,
			Env:          envMap,
			TrustedTools: srv.TrustedTools,
		})
	}

	return ConfigData{
		DefaultProvider:   defaultProvider,
		PreferredLanguage: answers.String("preferred_language", ""),
		Providers:         pdata,
		Embedding:         embData,
		LayeredContext:    lcData,
		MCPServers:        mcpData,
		GatewayHost:       answers.String("gateway_host", "127.0.0.1"),
		GatewayPort:       answers.Int("gateway_port", 18420),
	}
}

// --- JSON serialization types ---

type configJSON struct {
	Gateway        gatewayJSON         `json:"gateway"`
	Models         modelsJSON          `json:"models"`
	Embedding      *embeddingJSON      `json:"embedding,omitempty"`
	LayeredContext *layeredContextJSON  `json:"layered_context,omitempty"`
	MCP            *mcpJSON            `json:"mcp,omitempty"`
	Events         eventsJSON          `json:"events"`
	Agent          *agentJSON          `json:"agent,omitempty"`
}

type mcpJSON struct {
	Servers json.RawMessage `json:"servers,omitempty"`
}

type mcpServerJSON struct {
	Transport    string            `json:"transport"`
	Command      string            `json:"command,omitempty"`
	Args         []string          `json:"args,omitempty"`
	URL          string            `json:"url,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Dangerous    bool              `json:"dangerous"`
	TrustedTools []string          `json:"trusted_tools,omitempty"`
	Timeout      int               `json:"timeout"`
}

type layeredContextJSON struct {
	Enabled           bool `json:"enabled"`
	MaxRecentMessages int  `json:"max_recent_messages"`
	MaxArchives       int  `json:"max_archives"`
	ArchiveChunkSize  int  `json:"archive_chunk_size"`
}

type embeddingJSON struct {
	Enabled bool      `json:"enabled"`
	Driver  string    `json:"driver"`
	Model   string    `json:"model"`
	BaseURL string    `json:"base_url,omitempty"`
	Dims    int       `json:"dims,omitempty"`
	Auth    *authJSON `json:"auth,omitempty"`
}

type gatewayJSON struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type modelsJSON struct {
	Default   string          `json:"default"`
	Providers json.RawMessage `json:"providers"`
}

type providerJSON struct {
	Driver       string    `json:"driver"`
	Model        string    `json:"model"`
	BaseURL      string    `json:"base_url,omitempty"`
	Auth         *authJSON `json:"auth,omitempty"`
	Capabilities []string  `json:"capabilities,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	PromptPrefix string    `json:"prompt_prefix,omitempty"`
	MaxTokens    int       `json:"max_tokens"`
}

type authJSON struct {
	APIKey string `json:"api_key"`
}

type eventsJSON struct {
	BufferSize int `json:"buffer_size"`
}

type agentJSON struct {
	SystemPrompt      string `json:"system_prompt,omitempty"`
	PreferredLanguage string `json:"preferred_language,omitempty"`
}

// buildProvidersJSON builds an ordered JSON object from the provider list.
func buildProvidersJSON(providers []ProviderConfigData) (json.RawMessage, error) {
	// Use hujson to build an ordered object (maps lose insertion order).
	obj := &hujson.Object{}
	for _, p := range providers {
		pj := providerJSON{
			Driver:    p.Driver,
			Model:     p.Model,
			BaseURL:   p.BaseURL,
			MaxTokens: 4096,
		}
		if p.AuthEnvVar != "" {
			pj.Auth = &authJSON{APIKey: fmt.Sprintf("${{ .Env.%s }}", p.AuthEnvVar)}
		}
		if len(p.Capabilities) > 0 {
			pj.Capabilities = p.Capabilities
		}
		if len(p.Tags) > 0 {
			pj.Tags = p.Tags
		}
		if p.SystemPrompt != "" {
			pj.PromptPrefix = p.SystemPrompt
		}

		raw, err := json.Marshal(pj)
		if err != nil {
			return nil, fmt.Errorf("marshal provider %s: %w", p.Alias, err)
		}
		pVal, err := hujson.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse provider %s: %w", p.Alias, err)
		}

		obj.Members = append(obj.Members, hujson.ObjectMember{
			Name:  hujson.Value{Value: hujson.String(p.Alias)},
			Value: pVal,
		})
	}

	v := hujson.Value{Value: obj}
	return json.RawMessage(v.Pack()), nil
}

// buildMCPServersJSON builds an ordered JSON object from the MCP server list.
func buildMCPServersJSON(servers []MCPServerConfigData) (json.RawMessage, error) {
	obj := &hujson.Object{}
	for _, s := range servers {
		sj := mcpServerJSON{
			Transport: s.Transport,
			Command:   s.Command,
			URL:       s.URL,
			Dangerous: true,
			Timeout:   30000,
		}
		if len(s.Args) > 0 {
			sj.Args = s.Args
		}
		if len(s.Env) > 0 {
			sj.Env = s.Env
		}
		if len(s.TrustedTools) > 0 {
			sj.TrustedTools = s.TrustedTools
		}

		raw, err := json.Marshal(sj)
		if err != nil {
			return nil, fmt.Errorf("marshal mcp server %s: %w", s.Name, err)
		}
		pVal, err := hujson.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse mcp server %s: %w", s.Name, err)
		}

		obj.Members = append(obj.Members, hujson.ObjectMember{
			Name:  hujson.Value{Value: hujson.String(s.Name)},
			Value: pVal,
		})
	}

	v := hujson.Value{Value: obj}
	return json.RawMessage(v.Pack()), nil
}

// RenderConfig renders the JSONC config from the given data.
func RenderConfig(data ConfigData) (string, error) {
	provJSON, err := buildProvidersJSON(data.Providers)
	if err != nil {
		return "", fmt.Errorf("build providers: %w", err)
	}

	cfg := configJSON{
		Gateway: gatewayJSON{Host: data.GatewayHost, Port: data.GatewayPort},
		Models:  modelsJSON{Default: data.DefaultProvider, Providers: provJSON},
		Events:  eventsJSON{BufferSize: 1024},
	}

	if data.PreferredLanguage != "" {
		if cfg.Agent == nil {
			cfg.Agent = &agentJSON{}
		}
		cfg.Agent.PreferredLanguage = data.PreferredLanguage
	}

	if len(data.MCPServers) > 0 {
		mcpRaw, err := buildMCPServersJSON(data.MCPServers)
		if err != nil {
			return "", fmt.Errorf("build mcp servers: %w", err)
		}
		cfg.MCP = &mcpJSON{Servers: mcpRaw}
	}

	if data.LayeredContext != nil {
		cfg.LayeredContext = &layeredContextJSON{
			Enabled:           true,
			MaxRecentMessages: data.LayeredContext.MaxRecentMessages,
			MaxArchives:       data.LayeredContext.MaxArchives,
			ArchiveChunkSize:  8, // default, not exposed in wizard
		}
	}

	if data.Embedding != nil {
		ej := &embeddingJSON{
			Enabled: true,
			Driver:  data.Embedding.Driver,
			Model:   data.Embedding.Model,
			BaseURL: data.Embedding.BaseURL,
			Dims:    data.Embedding.Dims,
		}
		if data.Embedding.AuthEnvVar != "" {
			ej.Auth = &authJSON{APIKey: fmt.Sprintf("${{ .Env.%s }}", data.Embedding.AuthEnvVar)}
		}
		cfg.Embedding = ej
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	// Parse as hujson to add header comment and format.
	v, err := hujson.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse config: %w", err)
	}

	// Add header comment before the first key.
	if obj, ok := v.Value.(*hujson.Object); ok && len(obj.Members) > 0 {
		obj.Members[0].Name.BeforeExtra = hujson.Extra(
			"\n\t// Ozzie Configuration — generated by ozzie wake\n\n\t",
		)
	}

	v.Format()
	return string(v.Pack()), nil
}

// RenderConfigToFile is a convenience that writes the rendered config to path.
func RenderConfigToFile(data ConfigData, path string) error {
	content, err := RenderConfig(data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
