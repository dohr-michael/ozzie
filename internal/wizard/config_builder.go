package wizard

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tailscale/hujson"
)

// ConfigData holds the data for rendering the JSONC config.
type ConfigData struct {
	DefaultProvider string
	Providers       []ProviderConfigData
	GatewayHost     string
	GatewayPort     int
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
	"anthropic": "claude",
	"openai":    "gpt",
	"gemini":    "gemini",
	"mistral":   "mistral",
	"ollama":    "local",
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

	return ConfigData{
		DefaultProvider: defaultProvider,
		Providers:       pdata,
		GatewayHost:     answers.String("gateway_host", "127.0.0.1"),
		GatewayPort:     answers.Int("gateway_port", 18420),
	}
}

// --- JSON serialization types ---

type configJSON struct {
	Gateway  gatewayJSON  `json:"gateway"`
	Models   modelsJSON   `json:"models"`
	Events   eventsJSON   `json:"events"`
	Agent    agentJSON    `json:"agent"`
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
	Driver       string       `json:"driver"`
	Model        string       `json:"model"`
	BaseURL      string       `json:"base_url,omitempty"`
	Auth         *authJSON    `json:"auth,omitempty"`
	Capabilities []string     `json:"capabilities,omitempty"`
	Tags         []string     `json:"tags,omitempty"`
	PromptPrefix string       `json:"prompt_prefix,omitempty"`
	MaxTokens    int          `json:"max_tokens"`
}

type authJSON struct {
	APIKey string `json:"api_key"`
}

type eventsJSON struct {
	BufferSize int `json:"buffer_size"`
}

type agentJSON struct {
	SystemPrompt string `json:"system_prompt"`
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
		Agent:   agentJSON{SystemPrompt: ""},
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
