package setup_wizard

import (
	"encoding/json"
	"testing"

	"github.com/tailscale/hujson"
)

func TestAnswersString(t *testing.T) {
	a := Answers{"name": "ozzie"}
	if got := a.String("name", "x"); got != "ozzie" {
		t.Errorf("got %q, want %q", got, "ozzie")
	}
	if got := a.String("missing", "fallback"); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
	// Empty string should return fallback.
	a["empty"] = ""
	if got := a.String("empty", "fb"); got != "fb" {
		t.Errorf("got %q, want %q", got, "fb")
	}
}

func TestAnswersInt(t *testing.T) {
	a := Answers{"port": 8080, "float": 3.14}
	if got := a.Int("port", 0); got != 8080 {
		t.Errorf("got %d, want %d", got, 8080)
	}
	if got := a.Int("float", 0); got != 3 {
		t.Errorf("got %d, want %d", got, 3)
	}
	if got := a.Int("missing", 42); got != 42 {
		t.Errorf("got %d, want %d", got, 42)
	}
}

func TestAnswersBool(t *testing.T) {
	a := Answers{"ok": true}
	if got := a.Bool("ok", false); !got {
		t.Error("expected true")
	}
	if got := a.Bool("missing", true); !got {
		t.Error("expected true (fallback)")
	}
}

func TestAnswersMerge(t *testing.T) {
	a := Answers{"a": 1, "b": 2}
	b := Answers{"b": 3, "c": 4}
	a.Merge(b)

	if got := a.Int("a", 0); got != 1 {
		t.Errorf("a: got %d, want 1", got)
	}
	if got := a.Int("b", 0); got != 3 {
		t.Errorf("b: got %d, want 3 (should be overwritten)", got)
	}
	if got := a.Int("c", 0); got != 4 {
		t.Errorf("c: got %d, want 4", got)
	}
}

func TestBuildConfigData(t *testing.T) {
	answers := Answers{
		"providers": []ProviderEntry{
			{
				Alias:      "claude",
				Driver:     "anthropic",
				Model:      "claude-sonnet-4-20250514",
				EnvVarName: "ANTHROPIC_API_KEY",
			},
		},
		"default_provider": "claude",
		"gateway_host":     "0.0.0.0",
		"gateway_port":     9090,
	}

	data := BuildConfigData(answers)

	if data.DefaultProvider != "claude" {
		t.Errorf("DefaultProvider: got %q, want %q", data.DefaultProvider, "claude")
	}
	if len(data.Providers) != 1 {
		t.Fatalf("Providers: got %d, want 1", len(data.Providers))
	}
	p := data.Providers[0]
	if p.Driver != "anthropic" {
		t.Errorf("Driver: got %q, want %q", p.Driver, "anthropic")
	}
	if p.AuthEnvVar != "ANTHROPIC_API_KEY" {
		t.Errorf("AuthEnvVar: got %q, want %q", p.AuthEnvVar, "ANTHROPIC_API_KEY")
	}
	if data.GatewayHost != "0.0.0.0" {
		t.Errorf("GatewayHost: got %q, want %q", data.GatewayHost, "0.0.0.0")
	}
	if data.GatewayPort != 9090 {
		t.Errorf("GatewayPort: got %d, want %d", data.GatewayPort, 9090)
	}
}

func TestBuildConfigDataOllama(t *testing.T) {
	answers := Answers{
		"providers": []ProviderEntry{
			{
				Alias:   "local",
				Driver:  "ollama",
				Model:   "llama3.1:8b",
				BaseURL: "http://localhost:11434",
			},
		},
		"default_provider": "local",
	}

	data := BuildConfigData(answers)

	if data.DefaultProvider != "local" {
		t.Errorf("DefaultProvider: got %q, want %q", data.DefaultProvider, "local")
	}
	p := data.Providers[0]
	if p.AuthEnvVar != "" {
		t.Errorf("AuthEnvVar: got %q, want empty", p.AuthEnvVar)
	}
	if p.BaseURL != "http://localhost:11434" {
		t.Errorf("BaseURL: got %q, want %q", p.BaseURL, "http://localhost:11434")
	}
}

func TestRenderConfig(t *testing.T) {
	data := ConfigData{
		DefaultProvider: "claude",
		Providers: []ProviderConfigData{
			{
				Alias:      "claude",
				Driver:     "anthropic",
				Model:      "claude-sonnet-4-20250514",
				AuthEnvVar: "ANTHROPIC_API_KEY",
			},
		},
		GatewayHost: "127.0.0.1",
		GatewayPort: 18420,
	}

	content, err := RenderConfig(data)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	cfg := parseRendered(t, content)
	if cfg.Gateway.Host != "127.0.0.1" {
		t.Errorf("host = %q, want 127.0.0.1", cfg.Gateway.Host)
	}
	if cfg.Gateway.Port != 18420 {
		t.Errorf("port = %d, want 18420", cfg.Gateway.Port)
	}
	if cfg.Models.Default != "claude" {
		t.Errorf("default = %q, want claude", cfg.Models.Default)
	}
	p, ok := cfg.Models.Providers["claude"]
	if !ok {
		t.Fatal("provider 'claude' not found")
	}
	if p.Driver != "anthropic" {
		t.Errorf("driver = %q, want anthropic", p.Driver)
	}
	if p.Model != "claude-sonnet-4-20250514" {
		t.Errorf("model = %q, want claude-sonnet-4-20250514", p.Model)
	}
	if p.Auth == nil || !contains(p.Auth.APIKey, "ANTHROPIC_API_KEY") {
		t.Errorf("auth.api_key should reference ANTHROPIC_API_KEY, got %+v", p.Auth)
	}
	if p.MaxTokens != 4096 {
		t.Errorf("max_tokens = %d, want 4096", p.MaxTokens)
	}

	// Header comment should be present.
	if !contains(content, "// Ozzie Configuration") {
		t.Error("missing header comment")
	}
}

func TestRenderConfigOllamaNoAuth(t *testing.T) {
	data := ConfigData{
		DefaultProvider: "local",
		Providers: []ProviderConfigData{
			{
				Alias:   "local",
				Driver:  "ollama",
				Model:   "llama3.1:8b",
				BaseURL: "http://localhost:11434",
			},
		},
		GatewayHost: "127.0.0.1",
		GatewayPort: 18420,
	}

	content, err := RenderConfig(data)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	cfg := parseRendered(t, content)
	p := cfg.Models.Providers["local"]
	if p.Auth != nil {
		t.Error("ollama config should not contain auth")
	}
	if p.BaseURL != "http://localhost:11434" {
		t.Errorf("base_url = %q, want http://localhost:11434", p.BaseURL)
	}
}

func TestRenderConfigMultiProvider(t *testing.T) {
	data := ConfigData{
		DefaultProvider: "claude",
		Providers: []ProviderConfigData{
			{
				Alias:      "claude",
				Driver:     "anthropic",
				Model:      "claude-sonnet-4-20250514",
				AuthEnvVar: "ANTHROPIC_API_KEY",
			},
			{
				Alias:   "local",
				Driver:  "ollama",
				Model:   "llama3.1:8b",
				BaseURL: "http://localhost:11434",
			},
		},
		GatewayHost: "127.0.0.1",
		GatewayPort: 18420,
	}

	content, err := RenderConfig(data)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	cfg := parseRendered(t, content)
	if cfg.Models.Default != "claude" {
		t.Errorf("default = %q, want claude", cfg.Models.Default)
	}
	if len(cfg.Models.Providers) != 2 {
		t.Fatalf("got %d providers, want 2", len(cfg.Models.Providers))
	}

	claude := cfg.Models.Providers["claude"]
	if claude.Driver != "anthropic" {
		t.Errorf("claude.driver = %q, want anthropic", claude.Driver)
	}
	if claude.Auth == nil || !contains(claude.Auth.APIKey, "ANTHROPIC_API_KEY") {
		t.Error("claude should have ANTHROPIC_API_KEY auth")
	}

	local := cfg.Models.Providers["local"]
	if local.Driver != "ollama" {
		t.Errorf("local.driver = %q, want ollama", local.Driver)
	}
	if local.BaseURL != "http://localhost:11434" {
		t.Errorf("local.base_url = %q, want http://localhost:11434", local.BaseURL)
	}
	if local.Auth != nil {
		t.Error("ollama should not have auth")
	}
}

func TestRenderConfigWithSystemPrompt(t *testing.T) {
	data := ConfigData{
		DefaultProvider: "claude",
		Providers: []ProviderConfigData{
			{
				Alias:        "claude",
				Driver:       "anthropic",
				Model:        "claude-sonnet-4-20250514",
				AuthEnvVar:   "ANTHROPIC_API_KEY",
				SystemPrompt: "You are a helpful assistant.",
			},
		},
		GatewayHost: "127.0.0.1",
		GatewayPort: 18420,
	}

	content, err := RenderConfig(data)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	cfg := parseRendered(t, content)
	p := cfg.Models.Providers["claude"]
	if p.PromptPrefix != "You are a helpful assistant." {
		t.Errorf("prompt_prefix = %q, want 'You are a helpful assistant.'", p.PromptPrefix)
	}
}

func TestDriverProviderNames(t *testing.T) {
	tests := map[string]string{
		"anthropic": "claude",
		"openai":    "gpt",
		"gemini":    "gemini",
		"mistral":   "mistral",
		"ollama":    "local",
	}
	for driver, want := range tests {
		if got := driverProviderNames[driver]; got != want {
			t.Errorf("driverProviderNames[%q]: got %q, want %q", driver, got, want)
		}
	}
}

func TestDefaultModelForDriver(t *testing.T) {
	tests := map[string]string{
		"anthropic": "claude-sonnet-4-20250514",
		"openai":    "gpt-4o",
		"gemini":    "gemini-2.5-flash",
		"mistral":   "mistral-large-latest",
		"ollama":    "llama3.1:8b",
	}
	for driver, want := range tests {
		if got := defaultModelForDriver(driver); got != want {
			t.Errorf("defaultModelForDriver(%q): got %q, want %q", driver, got, want)
		}
	}
	// Unknown driver returns empty.
	if got := defaultModelForDriver("unknown"); got != "" {
		t.Errorf("defaultModelForDriver(unknown): got %q, want empty", got)
	}
}

func TestDriverModelsHaveCustomOption(t *testing.T) {
	for _, driver := range []string{"anthropic", "openai", "gemini", "mistral", "ollama"} {
		models := driverModelOptions(driver)
		last := models[len(models)-1]
		if last.Value != customModelValue {
			t.Errorf("driverModelOptions(%q): last option should be custom, got %q", driver, last.Value)
		}
	}
}

func TestAnswersStrings(t *testing.T) {
	a := Answers{"caps": []string{"coding", "vision"}}
	got := a.Strings("caps", nil)
	if len(got) != 2 || got[0] != "coding" || got[1] != "vision" {
		t.Errorf("got %v, want [coding vision]", got)
	}
	// Fallback.
	got = a.Strings("missing", []string{"default"})
	if len(got) != 1 || got[0] != "default" {
		t.Errorf("got %v, want [default]", got)
	}
	// Nil stored.
	a["nil"] = nil
	got = a.Strings("nil", []string{"fb"})
	if len(got) != 1 || got[0] != "fb" {
		t.Errorf("got %v, want [fb]", got)
	}
}

func TestAnswersProviders(t *testing.T) {
	providers := []ProviderEntry{
		{Alias: "claude", Driver: "anthropic"},
		{Alias: "local", Driver: "ollama"},
	}
	a := Answers{"providers": providers}
	got := a.Providers()
	if len(got) != 2 {
		t.Fatalf("got %d providers, want 2", len(got))
	}
	if got[0].Alias != "claude" {
		t.Errorf("got %q, want %q", got[0].Alias, "claude")
	}
	if got[1].Alias != "local" {
		t.Errorf("got %q, want %q", got[1].Alias, "local")
	}
	// Missing.
	a2 := Answers{}
	if a2.Providers() != nil {
		t.Error("expected nil for missing providers")
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"self-hosted, secured, primary", []string{"self-hosted", "secured", "primary"}},
		{"  a , b ,  ", []string{"a", "b"}},
		{"single", []string{"single"}},
		{"", nil},
		{" , , ", nil},
	}
	for _, tt := range tests {
		got := parseTags(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseTags(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseTags(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestRenderConfigWithCapabilitiesAndTags(t *testing.T) {
	data := ConfigData{
		DefaultProvider: "claude",
		Providers: []ProviderConfigData{
			{
				Alias:        "claude",
				Driver:       "anthropic",
				Model:        "claude-sonnet-4-20250514",
				AuthEnvVar:   "ANTHROPIC_API_KEY",
				Capabilities: []string{"tool_use", "coding"},
				Tags:         []string{"primary", "secured"},
			},
		},
		GatewayHost: "127.0.0.1",
		GatewayPort: 18420,
	}

	content, err := RenderConfig(data)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	cfg := parseRendered(t, content)
	p := cfg.Models.Providers["claude"]
	if len(p.Capabilities) != 2 || p.Capabilities[0] != "tool_use" || p.Capabilities[1] != "coding" {
		t.Errorf("capabilities = %v, want [tool_use, coding]", p.Capabilities)
	}
	if len(p.Tags) != 2 || p.Tags[0] != "primary" || p.Tags[1] != "secured" {
		t.Errorf("tags = %v, want [primary, secured]", p.Tags)
	}
}

func TestRenderConfigWithoutCapabilitiesAndTags(t *testing.T) {
	data := ConfigData{
		DefaultProvider: "claude",
		Providers: []ProviderConfigData{
			{
				Alias:      "claude",
				Driver:     "anthropic",
				Model:      "claude-sonnet-4-20250514",
				AuthEnvVar: "ANTHROPIC_API_KEY",
			},
		},
		GatewayHost: "127.0.0.1",
		GatewayPort: 18420,
	}

	content, err := RenderConfig(data)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	cfg := parseRendered(t, content)
	p := cfg.Models.Providers["claude"]
	if len(p.Capabilities) != 0 {
		t.Errorf("capabilities should be empty, got %v", p.Capabilities)
	}
	if len(p.Tags) != 0 {
		t.Errorf("tags should be empty, got %v", p.Tags)
	}
}

func TestDefaultCapsForKnownModels(t *testing.T) {
	// Every known model should have at least one capability.
	for model, caps := range modelDefaultCaps {
		if len(caps) == 0 {
			t.Errorf("modelDefaultCaps[%q] is empty", model)
		}
	}
	// Specific checks.
	sonnet := defaultCapsForModel("claude-sonnet-4-20250514")
	if sonnet == nil {
		t.Fatal("expected caps for claude-sonnet-4")
	}
	hasThinking := false
	for _, c := range sonnet {
		if c == "thinking" {
			hasThinking = true
		}
	}
	if !hasThinking {
		t.Error("claude-sonnet-4 should have thinking capability")
	}
}

func TestDefaultCapsForUnknownModel(t *testing.T) {
	if caps := defaultCapsForModel("some-unknown-model"); caps != nil {
		t.Errorf("expected nil for unknown model, got %v", caps)
	}
}

func TestAllDriverModelsHaveDefaultCaps(t *testing.T) {
	for _, driver := range []string{"anthropic", "openai", "gemini", "mistral", "ollama"} {
		opts := driverModelOptions(driver)
		for _, opt := range opts {
			if opt.Value == customModelValue {
				continue
			}
			caps := defaultCapsForModel(opt.Value)
			if caps == nil {
				t.Errorf("model %q (driver %s) has no default capabilities", opt.Value, driver)
			}
		}
	}
}

func TestCapabilityOptions(t *testing.T) {
	opts := capabilityOptions()
	if len(opts) != 8 {
		t.Errorf("expected 8 capability options, got %d", len(opts))
	}
	// Each should have a non-empty description.
	for _, o := range opts {
		if o.Description == "" {
			t.Errorf("capability %q has empty description", o.Value)
		}
	}
}

func TestDriverEnvVars(t *testing.T) {
	tests := map[string]string{
		"anthropic": "ANTHROPIC_API_KEY",
		"openai":    "OPENAI_API_KEY",
		"gemini":    "GOOGLE_API_KEY",
		"mistral":   "MISTRAL_API_KEY",
	}
	for driver, want := range tests {
		if got := driverEnvVars[driver]; got != want {
			t.Errorf("driverEnvVars[%q]: got %q, want %q", driver, got, want)
		}
	}
	// Ollama should not have an env var.
	if _, ok := driverEnvVars["ollama"]; ok {
		t.Error("ollama should not have an env var entry")
	}
}

func TestFindReusableKey(t *testing.T) {
	s := &providerStep{}

	// No providers yet → nil.
	s.providers = nil
	s.current = ProviderEntry{Driver: "gemini"}
	if got := s.findReusableKey(); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}

	// Provider exists with same driver and key → reusable.
	s.providers = []ProviderEntry{
		{Alias: "gemini", Driver: "gemini", EnvVarName: "GOOGLE_API_KEY", APIKey: "key123"},
	}
	if got := s.findReusableKey(); got == nil {
		t.Error("expected reusable provider, got nil")
	} else if got.Alias != "gemini" {
		t.Errorf("got alias %q, want %q", got.Alias, "gemini")
	}

	// Provider exists with same driver but skipped key → not reusable.
	s.providers = []ProviderEntry{
		{Alias: "gemini", Driver: "gemini", EnvVarName: "GOOGLE_API_KEY", SkipKey: true},
	}
	if got := s.findReusableKey(); got != nil {
		t.Errorf("skipped key should not be reusable, got %+v", got)
	}

	// Different driver → not reusable.
	s.providers = []ProviderEntry{
		{Alias: "claude", Driver: "anthropic", EnvVarName: "ANTHROPIC_API_KEY", APIKey: "key"},
	}
	s.current = ProviderEntry{Driver: "gemini"}
	if got := s.findReusableKey(); got != nil {
		t.Errorf("different driver should not be reusable, got %+v", got)
	}
}

func TestResolveEnvVarName(t *testing.T) {
	s := &providerStep{}

	// First gemini provider gets the base env var.
	s.providers = nil
	s.current = ProviderEntry{Driver: "gemini", Alias: "gemini"}
	if got := s.resolveEnvVarName(); got != "GOOGLE_API_KEY" {
		t.Errorf("first gemini: got %q, want GOOGLE_API_KEY", got)
	}

	// Second gemini provider (different alias) gets suffixed.
	s.providers = []ProviderEntry{
		{Driver: "gemini", Alias: "gemini", EnvVarName: "GOOGLE_API_KEY"},
	}
	s.current = ProviderEntry{Driver: "gemini", Alias: "gemini-2"}
	if got := s.resolveEnvVarName(); got != "GOOGLE_API_KEY_GEMINI_2" {
		t.Errorf("second gemini: got %q, want GOOGLE_API_KEY_GEMINI_2", got)
	}

	// Ollama has no env var.
	s.current = ProviderEntry{Driver: "ollama", Alias: "local"}
	if got := s.resolveEnvVarName(); got != "" {
		t.Errorf("ollama: got %q, want empty", got)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && containsStr(s, substr)
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// parsedConfig is used by tests to verify rendered config semantically.
type parsedConfig struct {
	Gateway struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"gateway"`
	Models struct {
		Default   string                          `json:"default"`
		Providers map[string]parsedConfigProvider `json:"providers"`
	} `json:"models"`
	Events struct {
		BufferSize int `json:"buffer_size"`
	} `json:"events"`
	Agent *struct {
		SystemPrompt string `json:"system_prompt"`
	} `json:"agent,omitempty"`
}

type parsedConfigProvider struct {
	Driver  string `json:"driver"`
	Model   string `json:"model"`
	BaseURL string `json:"base_url,omitempty"`
	Auth    *struct {
		APIKey string `json:"api_key"`
	} `json:"auth,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	PromptPrefix string   `json:"prompt_prefix,omitempty"`
	MaxTokens    int      `json:"max_tokens"`
}

// parseRendered standardizes hujson output and unmarshals into parsedConfig.
func parseRendered(t *testing.T, content string) parsedConfig {
	t.Helper()
	std, err := hujson.Standardize([]byte(content))
	if err != nil {
		t.Fatalf("Standardize rendered config: %v\nContent:\n%s", err, content)
	}
	var cfg parsedConfig
	if err := json.Unmarshal(std, &cfg); err != nil {
		t.Fatalf("Unmarshal rendered config: %v\nJSON:\n%s", err, std)
	}
	return cfg
}
