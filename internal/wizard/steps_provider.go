package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dohr-michael/ozzie/clients/tui/components"
	"github.com/dohr-michael/ozzie/internal/models"
)

const customModelValue = "__custom__"

// driverModels lists popular models per driver, first entry is the default.
var driverModels = map[string][]components.InputOption{
	"anthropic": {
		{Value: "claude-sonnet-4-20250514", Label: "Claude Sonnet 4", Description: "Best balance of speed and quality"},
		{Value: "claude-opus-4-20250514", Label: "Claude Opus 4", Description: "Most capable"},
		{Value: "claude-haiku-4-20250414", Label: "Claude Haiku 4", Description: "Fast and affordable"},
		{Value: customModelValue, Label: "Custom model", Description: "Enter a model ID manually"},
	},
	"openai": {
		{Value: "gpt-4o", Label: "GPT-4o", Description: "Flagship multimodal model"},
		{Value: "gpt-4o-mini", Label: "GPT-4o Mini", Description: "Fast and affordable"},
		{Value: "o3", Label: "o3", Description: "Reasoning model"},
		{Value: customModelValue, Label: "Custom model", Description: "Enter a model ID manually"},
	},
	"gemini": {
		{Value: "gemini-2.5-flash", Label: "Gemini 2.5 Flash", Description: "Fast and versatile"},
		{Value: "gemini-2.5-pro", Label: "Gemini 2.5 Pro", Description: "Most capable"},
		{Value: customModelValue, Label: "Custom model", Description: "Enter a model ID manually"},
	},
	"mistral": {
		{Value: "mistral-large-latest", Label: "Mistral Large", Description: "Most capable"},
		{Value: "mistral-medium-latest", Label: "Mistral Medium", Description: "Balanced"},
		{Value: "mistral-small-latest", Label: "Mistral Small", Description: "Fast and affordable"},
		{Value: customModelValue, Label: "Custom model", Description: "Enter a model ID manually"},
	},
	"ollama": {
		{Value: "llama3.1:8b", Label: "Llama 3.1 8B", Description: "Good general purpose"},
		{Value: "qwen2.5-coder:7b", Label: "Qwen 2.5 Coder 7B", Description: "Optimized for code"},
		{Value: "deepseek-r1:8b", Label: "DeepSeek R1 8B", Description: "Reasoning model"},
		{Value: "mistral:7b", Label: "Mistral 7B", Description: "Fast and capable"},
		{Value: customModelValue, Label: "Custom model", Description: "Enter a model ID manually"},
	},
}

// defaultModelForDriver returns the first (default) model for a driver.
func defaultModelForDriver(driver string) string {
	m := driverModels[driver]
	if len(m) > 0 {
		return m[0].Value
	}
	return ""
}

// capabilityOptions builds InputOption list from models.AllCapabilities.
func capabilityOptions() []components.InputOption {
	descs := map[models.Capability]string{
		models.CapThinking:    "Extended thinking / chain-of-thought",
		models.CapVision:      "Image / multimodal input",
		models.CapToolUse:     "Function / tool calling",
		models.CapCoding:      "Code generation optimized",
		models.CapLongContext: ">100K token context",
		models.CapFast:        "Low-latency inference",
		models.CapCheap:       "Cost-optimized",
		models.CapWriting:     "Text / content generation",
	}
	opts := make([]components.InputOption, 0, len(models.AllCapabilities))
	for _, c := range models.AllCapabilities {
		opts = append(opts, components.InputOption{
			Value:       string(c),
			Label:       string(c),
			Description: descs[c],
		})
	}
	return opts
}

// modelDefaultCaps maps known model IDs to their default capabilities.
var modelDefaultCaps = map[string][]string{
	// Anthropic
	"claude-sonnet-4-20250514": {"thinking", "vision", "tool_use", "coding", "fast"},
	"claude-opus-4-20250514":   {"thinking", "vision", "tool_use", "coding", "long_context", "writing"},
	"claude-haiku-4-20250414":  {"vision", "tool_use", "coding", "fast", "cheap"},

	// OpenAI
	"gpt-4o":      {"vision", "tool_use", "coding", "fast"},
	"gpt-4o-mini": {"vision", "tool_use", "coding", "fast", "cheap"},
	"o3":          {"thinking", "vision", "tool_use", "coding"},

	// Gemini
	"gemini-2.5-flash": {"thinking", "vision", "tool_use", "coding", "fast", "cheap", "long_context"},
	"gemini-2.5-pro":   {"thinking", "vision", "tool_use", "coding", "long_context", "writing"},

	// Mistral
	"mistral-large-latest":  {"vision", "tool_use", "coding", "writing"},
	"mistral-medium-latest": {"vision", "tool_use", "coding"},
	"mistral-small-latest":  {"vision", "tool_use", "coding", "fast", "cheap"},

	// Ollama (local)
	"llama3.1:8b":       {"tool_use", "writing"},
	"qwen2.5-coder:7b":  {"coding", "tool_use"},
	"deepseek-r1:8b":    {"thinking", "coding"},
	"mistral:7b":        {"tool_use", "fast", "cheap"},
}

// defaultCapsForModel returns the default capabilities for a known model, or nil.
func defaultCapsForModel(model string) []string {
	return modelDefaultCaps[model]
}

// driversWithBaseURL lists drivers that support a custom base_url.
var driversWithBaseURL = map[string]bool{
	"ollama": true,
	"openai": true,
}

// Provider step phases:
//
//	0 = driver select
//	1 = model select
//	2 = custom model text
//	3 = base_url (ollama, or openai-compatible custom)
//	4 = capabilities (multi-select)
//	5 = tags (text input)
type providerStep struct {
	input        *components.InputZone
	phase        int
	driver       string
	model        string
	customModel  bool // true if the user typed a custom model ID
	baseURL      string
	capabilities []string
	tags         []string
}

func newProviderStep() *providerStep {
	return &providerStep{
		input: components.NewInputZone(),
	}
}

func (s *providerStep) ID() string    { return "provider" }
func (s *providerStep) Title() string { return "LLM Provider" }

func (s *providerStep) ShouldSkip(_ Answers) bool { return false }

func (s *providerStep) Init(answers Answers) tea.Cmd {
	s.phase = 0
	s.driver = answers.String("driver", "")
	s.model = answers.String("model", "")
	s.customModel = false
	s.baseURL = answers.String("base_url", "")
	s.capabilities = answers.Strings("capabilities", nil)
	s.tags = answers.Strings("tags", nil)
	s.input.ClearCompletedFields()
	s.showDriverSelect()
	return nil
}

func (s *providerStep) showDriverSelect() {
	s.input.PromptSelect(
		"Choose your LLM provider",
		"driver", "",
		[]components.InputOption{
			{Value: "anthropic", Label: "Anthropic", Description: "Claude models, best for coding"},
			{Value: "openai", Label: "OpenAI", Description: "GPT models"},
			{Value: "gemini", Label: "Google Gemini", Description: "Gemini models"},
			{Value: "mistral", Label: "Mistral", Description: "Mistral models, EU-based"},
			{Value: "ollama", Label: "Ollama", Description: "Local models, no API key needed"},
		},
		s.driver,
	)
}

func (s *providerStep) showModelSelect() {
	s.input.AddCompletedField("Provider", s.driver)

	options := driverModels[s.driver]
	defaultVal := ""
	if s.model != "" {
		defaultVal = s.model
	}
	s.input.PromptSelect(
		"Choose a model",
		"model", "",
		options,
		defaultVal,
	)
}

func (s *providerStep) showCustomModelInput() {
	s.input.AddCompletedField("Provider", s.driver)
	s.input.PromptText(
		"Enter model ID",
		"model", "", "", true, "",
	)
}

func (s *providerStep) showBaseURLInput() {
	prefill := s.baseURL

	label := "Base URL"
	placeholder := ""
	switch s.driver {
	case "ollama":
		label = "Ollama base URL"
		if prefill == "" {
			prefill = "http://localhost:11434"
		}
	case "openai":
		label = "Base URL (optional, for OpenAI-compatible APIs)"
		placeholder = "https://api.openai.com/v1 (leave empty for default)"
	}

	s.input.AddCompletedField("Model", s.model)
	s.input.PromptText(
		label,
		"base_url", prefill, placeholder, false, "",
	)
}

func (s *providerStep) addModelCompletedFields() {
	s.input.AddCompletedField("Provider", s.driver)
	s.input.AddCompletedField("Model", s.model)
	if s.baseURL != "" {
		s.input.AddCompletedField("Base URL", s.baseURL)
	}
}

func (s *providerStep) showCapabilities() {
	s.addModelCompletedFields()
	s.input.PromptMulti(
		"Capabilities (space=toggle, enter=confirm)",
		"capabilities", "",
		capabilityOptions(),
	)
	// Pre-select: explicit capabilities (edit mode) take priority, else model defaults.
	preselect := s.capabilities
	if len(preselect) == 0 {
		preselect = defaultCapsForModel(s.model)
	}
	if len(preselect) > 0 {
		s.input.PreSelectMulti(preselect)
	}
}

func (s *providerStep) showTags() {
	s.addModelCompletedFields()
	if len(s.capabilities) > 0 {
		s.input.AddCompletedField("Capabilities", strings.Join(s.capabilities, ", "))
	}
	s.input.PromptText(
		"Tags (optional, comma-separated — e.g. self-hosted, secured, primary)",
		"tags", strings.Join(s.tags, ", "), "", false, "",
	)
}

func (s *providerStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	// Handle back on esc.
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		switch s.phase {
		case 0:
			return s, func() tea.Msg { return StepBackMsg{} }
		case 1: // Model select → driver.
			s.phase = 0
			s.input.ClearCompletedFields()
			s.showDriverSelect()
			return s, nil
		case 2: // Custom model text → model select.
			s.phase = 1
			s.input.ClearCompletedFields()
			s.showModelSelect()
			return s, nil
		case 3: // Base URL → back to custom model text or model select.
			if s.customModel {
				s.phase = 2
				s.input.ClearCompletedFields()
				s.showCustomModelInput()
			} else {
				s.phase = 1
				s.input.ClearCompletedFields()
				s.showModelSelect()
			}
			return s, nil
		case 4: // Capabilities → back to base_url if needed, else model select.
			if s.needsBaseURL() {
				s.phase = 3
				s.input.ClearCompletedFields()
				s.input.AddCompletedField("Provider", s.driver)
				s.showBaseURLInput()
			} else {
				s.phase = 1
				s.input.ClearCompletedFields()
				s.showModelSelect()
			}
			return s, nil
		case 5: // Tags → capabilities.
			s.phase = 4
			s.input.ClearCompletedFields()
			s.showCapabilities()
			return s, nil
		}
	}

	// Handle InputResult.
	if result, ok := msg.(components.InputResult); ok {
		switch s.phase {
		case 0: // Driver selected.
			s.driver = result.Selected
			s.model = ""
			s.phase = 1
			s.showModelSelect()
			return s, nil

		case 1: // Model selected from list.
			if result.Selected == customModelValue {
				s.customModel = true
				s.phase = 2
				s.input.ClearCompletedFields()
				s.showCustomModelInput()
				return s, nil
			}
			s.customModel = false
			s.model = result.Selected
			return s, s.afterModelSelected()

		case 2: // Custom model entered.
			s.model = result.Text
			return s, s.afterModelSelected()

		case 3: // Base URL entered.
			text := result.Text
			if text == "" && s.driver == "ollama" {
				text = "http://localhost:11434"
			}
			s.baseURL = text
			s.phase = 4
			s.input.ClearCompletedFields()
			s.showCapabilities()
			return s, nil

		case 4: // Capabilities selected.
			s.capabilities = result.MultiSelect
			s.phase = 5
			s.input.ClearCompletedFields()
			s.showTags()
			return s, nil

		case 5: // Tags entered.
			s.tags = parseTags(result.Text)
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
	}

	input, cmd := s.input.Update(msg)
	s.input = input
	return s, cmd
}

// needsBaseURL returns true if this driver/model combo should prompt for a base URL.
// Ollama always needs one; OpenAI needs one only for custom models (OpenAI-compatible APIs).
func (s *providerStep) needsBaseURL() bool {
	if s.driver == "ollama" {
		return true
	}
	return s.customModel && driversWithBaseURL[s.driver]
}

// afterModelSelected transitions to base_url or capabilities.
func (s *providerStep) afterModelSelected() tea.Cmd {
	if s.needsBaseURL() {
		s.phase = 3
		s.input.ClearCompletedFields()
		s.input.AddCompletedField("Provider", s.driver)
		s.showBaseURLInput()
		return nil
	}
	s.phase = 4
	s.input.ClearCompletedFields()
	s.showCapabilities()
	return nil
}

func (s *providerStep) View() string {
	return s.input.View()
}

func (s *providerStep) Collect() Answers {
	a := Answers{
		"driver":       s.driver,
		"model":        s.model,
		"capabilities": s.capabilities,
		"tags":         s.tags,
	}
	if s.baseURL != "" {
		a["base_url"] = s.baseURL
	}
	return a
}

// parseTags splits a comma-separated string into trimmed, non-empty tags.
func parseTags(raw string) []string {
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}
