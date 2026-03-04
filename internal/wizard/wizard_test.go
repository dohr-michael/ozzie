package wizard

import (
	"testing"
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
		"driver":       "anthropic",
		"model":        "claude-sonnet-4-20250514",
		"gateway_host": "0.0.0.0",
		"gateway_port": 9090,
	}

	data := BuildConfigData(answers)

	if data.ProviderName != "claude" {
		t.Errorf("ProviderName: got %q, want %q", data.ProviderName, "claude")
	}
	if data.Driver != "anthropic" {
		t.Errorf("Driver: got %q, want %q", data.Driver, "anthropic")
	}
	if data.AuthEnvVar != "ANTHROPIC_API_KEY" {
		t.Errorf("AuthEnvVar: got %q, want %q", data.AuthEnvVar, "ANTHROPIC_API_KEY")
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
		"driver":   "ollama",
		"model":    "llama3.1:8b",
		"base_url": "http://localhost:11434",
	}

	data := BuildConfigData(answers)

	if data.ProviderName != "local" {
		t.Errorf("ProviderName: got %q, want %q", data.ProviderName, "local")
	}
	if data.AuthEnvVar != "" {
		t.Errorf("AuthEnvVar: got %q, want empty", data.AuthEnvVar)
	}
	if data.BaseURL != "http://localhost:11434" {
		t.Errorf("BaseURL: got %q, want %q", data.BaseURL, "http://localhost:11434")
	}
}

func TestRenderConfig(t *testing.T) {
	data := ConfigData{
		ProviderName: "claude",
		Driver:       "anthropic",
		Model:        "claude-sonnet-4-20250514",
		AuthEnvVar:   "ANTHROPIC_API_KEY",
		GatewayHost:  "127.0.0.1",
		GatewayPort:  18420,
	}

	content, err := RenderConfig(data)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	// Check key content is present.
	checks := []string{
		`"host": "127.0.0.1"`,
		`"port": 18420`,
		`"default": "claude"`,
		`"driver": "anthropic"`,
		`"model": "claude-sonnet-4-20250514"`,
		`ANTHROPIC_API_KEY`,
		`"max_tokens": 4096`,
	}
	for _, check := range checks {
		if !contains(content, check) {
			t.Errorf("config missing %q", check)
		}
	}
}

func TestRenderConfigOllamaNoAuth(t *testing.T) {
	data := ConfigData{
		ProviderName: "local",
		Driver:       "ollama",
		Model:        "llama3.1:8b",
		BaseURL:      "http://localhost:11434",
		GatewayHost:  "127.0.0.1",
		GatewayPort:  18420,
	}

	content, err := RenderConfig(data)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	if contains(content, "api_key") {
		t.Error("ollama config should not contain api_key")
	}
	if !contains(content, "base_url") {
		t.Error("ollama config should contain base_url")
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
	for driver, models := range driverModels {
		last := models[len(models)-1]
		if last.Value != customModelValue {
			t.Errorf("driverModels[%q]: last option should be custom, got %q", driver, last.Value)
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
		ProviderName: "claude",
		Driver:       "anthropic",
		Model:        "claude-sonnet-4-20250514",
		AuthEnvVar:   "ANTHROPIC_API_KEY",
		Capabilities: []string{"tool_use", "coding"},
		Tags:         []string{"primary", "secured"},
		GatewayHost:  "127.0.0.1",
		GatewayPort:  18420,
	}

	content, err := RenderConfig(data)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	checks := []string{
		`"capabilities": ["tool_use", "coding"]`,
		`"tags": ["primary", "secured"]`,
	}
	for _, check := range checks {
		if !contains(content, check) {
			t.Errorf("config missing %q\nGot:\n%s", check, content)
		}
	}
}

func TestRenderConfigWithoutCapabilitiesAndTags(t *testing.T) {
	data := ConfigData{
		ProviderName: "claude",
		Driver:       "anthropic",
		Model:        "claude-sonnet-4-20250514",
		AuthEnvVar:   "ANTHROPIC_API_KEY",
		GatewayHost:  "127.0.0.1",
		GatewayPort:  18420,
	}

	content, err := RenderConfig(data)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}

	if contains(content, "capabilities") {
		t.Error("config should not contain capabilities when empty")
	}
	if contains(content, `"tags"`) {
		t.Error("config should not contain tags when empty")
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
	for driver, opts := range driverModels {
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
