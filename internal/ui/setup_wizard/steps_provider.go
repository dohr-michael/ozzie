package setup_wizard

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/dohr-michael/ozzie/internal/i18n"
	"github.com/dohr-michael/ozzie/internal/infra/models"
	"github.com/dohr-michael/ozzie/internal/ui/components"
)

const customModelValue = "__custom__"

// Phase constants for provider configuration.
const (
	phaseDriver       = iota // select driver
	phaseAlias               // text input for alias
	phaseModel               // select model
	phaseCustomModel         // text input for custom model ID
	phaseBaseURL             // text input for base URL
	phaseAPIKeyAsk           // select: enter now / set later
	phaseAPIKeyInput         // password text input
	phaseCaps                // multi-select capabilities
	phaseTags                // text input for tags
	phaseSystemPrompt        // text input for system prompt
	phaseAddMore             // confirm: add another provider?
)

// driverModelDefaults maps driver → list of {value, label, i18n key} for default ordering.
// Labels are proper names (not translated), descriptions come from i18n.
var driverModelDefaults = map[string][]struct {
	Value, Label, DescKey string
}{
	"anthropic": {
		{"claude-sonnet-4-20250514", "Claude Sonnet 4", "wizard.model.sonnet4.desc"},
		{"claude-opus-4-20250514", "Claude Opus 4", "wizard.model.opus4.desc"},
		{"claude-haiku-4-20250414", "Claude Haiku 4", "wizard.model.haiku4.desc"},
	},
	"openai": {
		{"gpt-4o", "GPT-4o", "wizard.model.gpt4o.desc"},
		{"gpt-4o-mini", "GPT-4o Mini", "wizard.model.gpt4omini.desc"},
		{"o3", "o3", "wizard.model.o3.desc"},
	},
	"openai-like": {},
	"gemini": {
		{"gemini-2.5-flash", "Gemini 2.5 Flash", "wizard.model.gem25flash.desc"},
		{"gemini-2.5-pro", "Gemini 2.5 Pro", "wizard.model.gem25pro.desc"},
	},
	"mistral": {
		{"mistral-large-latest", "Mistral Large", "wizard.model.mistral_lg.desc"},
		{"mistral-medium-latest", "Mistral Medium", "wizard.model.mistral_md.desc"},
		{"mistral-small-latest", "Mistral Small", "wizard.model.mistral_sm.desc"},
	},
	"ollama": {
		{"llama3.1:8b", "Llama 3.1 8B", "wizard.model.llama31.desc"},
		{"qwen2.5-coder:7b", "Qwen 2.5 Coder 7B", "wizard.model.qwen25.desc"},
		{"deepseek-r1:8b", "DeepSeek R1 8B", "wizard.model.deepseek.desc"},
		{"mistral:7b", "Mistral 7B", "wizard.model.mistral7b.desc"},
	},
}

// driverModelOptions returns the model options for a driver with translated descriptions.
func driverModelOptions(driver string) []components.InputOption {
	defs := driverModelDefaults[driver]
	opts := make([]components.InputOption, 0, len(defs)+1)
	for _, d := range defs {
		opts = append(opts, components.InputOption{
			Value:       d.Value,
			Label:       d.Label,
			Description: i18n.T(d.DescKey),
		})
	}
	opts = append(opts, components.InputOption{
		Value:       customModelValue,
		Label:       i18n.T("wizard.provider.custom"),
		Description: i18n.T("wizard.provider.custom.desc"),
	})
	return opts
}

// defaultModelForDriver returns the first (default) model for a driver.
func defaultModelForDriver(driver string) string {
	m := driverModelDefaults[driver]
	if len(m) > 0 {
		return m[0].Value
	}
	return ""
}

// driversWithBaseURL lists drivers that support a custom base_url.
var driversWithBaseURL = map[string]bool{
	"ollama":      true,
	"openai-like": true,
}

// modelDefaultCaps maps known model IDs to their default capabilities.
var modelDefaultCaps = map[string][]string{
	"claude-sonnet-4-20250514": {"thinking", "vision", "tool_use", "coding", "fast"},
	"claude-opus-4-20250514":   {"thinking", "vision", "tool_use", "coding", "long_context", "writing"},
	"claude-haiku-4-20250414":  {"vision", "tool_use", "coding", "fast", "cheap"},
	"gpt-4o":                   {"vision", "tool_use", "coding", "fast"},
	"gpt-4o-mini":              {"vision", "tool_use", "coding", "fast", "cheap"},
	"o3":                       {"thinking", "vision", "tool_use", "coding"},
	"gemini-2.5-flash":         {"thinking", "vision", "tool_use", "coding", "fast", "cheap", "long_context"},
	"gemini-2.5-pro":           {"thinking", "vision", "tool_use", "coding", "long_context", "writing"},
	"mistral-large-latest":     {"vision", "tool_use", "coding", "writing"},
	"mistral-medium-latest":    {"vision", "tool_use", "coding"},
	"mistral-small-latest":     {"vision", "tool_use", "coding", "fast", "cheap"},
	"llama3.1:8b":              {"tool_use", "writing"},
	"qwen2.5-coder:7b":         {"coding", "tool_use"},
	"deepseek-r1:8b":           {"thinking", "coding"},
	"mistral:7b":               {"tool_use", "fast", "cheap"},
}

// defaultCapsForModel returns the default capabilities for a known model, or nil.
func defaultCapsForModel(model string) []string {
	return modelDefaultCaps[model]
}

// capDescKeys maps capabilities to their i18n description keys.
var capDescKeys = map[models.Capability]string{
	models.CapThinking:    "wizard.cap.thinking.desc",
	models.CapVision:      "wizard.cap.vision.desc",
	models.CapToolUse:     "wizard.cap.tool_use.desc",
	models.CapCoding:      "wizard.cap.coding.desc",
	models.CapLongContext: "wizard.cap.long_context.desc",
	models.CapFast:        "wizard.cap.fast.desc",
	models.CapCheap:       "wizard.cap.cheap.desc",
	models.CapWriting:     "wizard.cap.writing.desc",
}

// capabilityOptions builds InputOption list from models.AllCapabilities with translated descriptions.
func capabilityOptions() []components.InputOption {
	opts := make([]components.InputOption, 0, len(models.AllCapabilities))
	for _, c := range models.AllCapabilities {
		opts = append(opts, components.InputOption{
			Value:       string(c),
			Label:       string(c),
			Description: i18n.T(capDescKeys[c]),
		})
	}
	return opts
}

type providerStep struct {
	input       *components.InputZone
	keyInput    textinput.Model
	phase       int
	customModel bool
	useKeyInput bool // true when in password input mode

	current   ProviderEntry   // provider being configured
	providers []ProviderEntry // completed providers
}

func newProviderStep() *providerStep {
	ti := textinput.New()
	ti.Prompt = "  "
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	ti.CharLimit = 200
	ti.SetWidth(60)
	ti.Placeholder = "Paste your API key..."

	return &providerStep{
		input:    components.NewInputZone(),
		keyInput: ti,
	}
}

func (s *providerStep) ID() string    { return "provider" }
func (s *providerStep) Title() string { return "LLM Providers" }

func (s *providerStep) ShouldSkip(_ Answers) bool { return false }

func (s *providerStep) Init(answers Answers) tea.Cmd {
	s.providers = answers.Providers()
	s.current = ProviderEntry{}
	s.customModel = false
	s.useKeyInput = false

	if len(s.providers) > 0 {
		// Edit mode: show existing, ask to add more.
		s.phase = phaseAddMore
		s.showPhase()
		return nil
	}

	s.phase = phaseDriver
	s.showPhase()
	return nil
}

// --- Phase display methods ---

func (s *providerStep) showPhase() {
	s.input.ClearCompletedFields()
	s.useKeyInput = false

	// Show already-configured providers.
	if len(s.providers) > 0 {
		names := make([]string, len(s.providers))
		for i, p := range s.providers {
			names[i] = p.Alias
		}
		s.input.AddCompletedField(i18n.T("wizard.field.configured"), strings.Join(names, ", "))
	}

	switch s.phase {
	case phaseDriver:
		s.showDriverSelect()
	case phaseAlias:
		s.addCurrentProgress()
		s.showAliasInput()
	case phaseModel:
		s.addCurrentProgress()
		s.showModelSelect()
	case phaseCustomModel:
		s.addCurrentProgress()
		s.showCustomModelInput()
	case phaseBaseURL:
		s.addCurrentProgress()
		s.showBaseURLInput()
	case phaseAPIKeyAsk:
		s.addCurrentProgress()
		s.showAPIKeyAsk()
	case phaseAPIKeyInput:
		s.addCurrentProgress()
		s.showAPIKeyInput()
	case phaseCaps:
		s.addCurrentProgress()
		s.showCapabilities()
	case phaseTags:
		s.addCurrentProgress()
		s.showTags()
	case phaseSystemPrompt:
		s.addCurrentProgress()
		s.showSystemPrompt()
	case phaseAddMore:
		s.showAddMore()
	}
}

// addCurrentProgress adds completed fields for the current provider being configured.
func (s *providerStep) addCurrentProgress() {
	if s.current.Driver != "" && s.phase > phaseDriver {
		s.input.AddCompletedField(i18n.T("wizard.field.driver"), s.current.Driver)
	}
	if s.current.Alias != "" && s.phase > phaseAlias {
		s.input.AddCompletedField(i18n.T("wizard.field.alias"), s.current.Alias)
	}
	if s.current.Model != "" && s.phase > phaseModel && s.phase > phaseCustomModel {
		s.input.AddCompletedField(i18n.T("wizard.field.model"), s.current.Model)
	}
	if s.current.BaseURL != "" && s.phase > phaseBaseURL {
		s.input.AddCompletedField(i18n.T("wizard.field.base_url"), s.current.BaseURL)
	}
	if len(s.current.Capabilities) > 0 && s.phase > phaseCaps {
		s.input.AddCompletedField(i18n.T("wizard.field.caps"), strings.Join(s.current.Capabilities, ", "))
	}
	if s.current.Tags != nil && s.phase > phaseTags {
		if len(s.current.Tags) > 0 {
			s.input.AddCompletedField(i18n.T("wizard.field.tags"), strings.Join(s.current.Tags, ", "))
		}
	}
}

func (s *providerStep) showDriverSelect() {
	s.input.PromptSelect(
		i18n.T("wizard.provider.choose"),
		"driver", "",
		[]components.InputOption{
			{Value: "anthropic", Label: "Anthropic", Description: i18n.T("wizard.driver.anthropic.desc")},
			{Value: "openai", Label: "OpenAI", Description: i18n.T("wizard.driver.openai.desc")},
			{Value: "openai-like", Label: "OpenAI Compatible", Description: i18n.T("wizard.driver.openai-like.desc")},
			{Value: "gemini", Label: "Google Gemini", Description: i18n.T("wizard.driver.gemini.desc")},
			{Value: "mistral", Label: "Mistral", Description: i18n.T("wizard.driver.mistral.desc")},
			{Value: "ollama", Label: "Ollama", Description: i18n.T("wizard.driver.ollama.desc")},
		},
		"",
	)
}

func (s *providerStep) showAliasInput() {
	s.input.PromptText(
		i18n.T("wizard.provider.alias"),
		"alias", s.defaultAlias(), "", false, "",
	)
}

func (s *providerStep) showModelSelect() {
	options := driverModelOptions(s.current.Driver)
	s.input.PromptSelect(i18n.T("wizard.provider.model"), "model", "", options, "")
}

func (s *providerStep) showCustomModelInput() {
	s.input.PromptText(i18n.T("wizard.provider.model_id"), "model", "", "", true, "")
}

func (s *providerStep) showBaseURLInput() {
	label := i18n.T("wizard.provider.base_url")
	placeholder := ""
	switch s.current.Driver {
	case "ollama":
		label = i18n.T("wizard.provider.base_url.ollama")
		if s.current.BaseURL == "" {
			placeholder = "http://localhost:11434"
		} else {
			placeholder = s.current.BaseURL
		}
	case "openai-like":
		label = i18n.T("wizard.provider.base_url.openai-like")
		if s.current.BaseURL == "" {
			placeholder = "http://localhost:8080/v1"
		} else {
			placeholder = s.current.BaseURL
		}
	}
	s.input.PromptText(label, "base_url", placeholder, "", false, "")
}

func (s *providerStep) showAPIKeyAsk() {
	envVar := s.current.EnvVarName

	opts := []components.InputOption{
		{Value: "now", Label: i18n.T("wizard.provider.key_now"), Description: i18n.T("wizard.provider.key_now.desc")},
		{Value: "later", Label: i18n.T("wizard.provider.key_later"), Description: fmt.Sprintf(i18n.T("wizard.provider.key_later.desc"), envVar)},
	}
	defaultOpt := "now"

	// If another provider with the same driver already has a key, offer reuse.
	if existing := s.findReusableKey(); existing != nil {
		opts = []components.InputOption{
			{Value: "reuse", Label: fmt.Sprintf(i18n.T("wizard.provider.key_reuse"), existing.Alias), Description: fmt.Sprintf(i18n.T("wizard.provider.key_reuse.desc"), existing.EnvVarName)},
			{Value: "now", Label: i18n.T("wizard.provider.key_new"), Description: fmt.Sprintf(i18n.T("wizard.provider.key_new.desc"), envVar)},
			{Value: "later", Label: i18n.T("wizard.provider.key_later"), Description: fmt.Sprintf(i18n.T("wizard.provider.key_later.desc"), envVar)},
		}
		defaultOpt = "reuse"
	}

	s.input.PromptSelect(fmt.Sprintf(i18n.T("wizard.provider.api_key_for"), envVar), "ask_key", "", opts, defaultOpt)
}

func (s *providerStep) showAPIKeyInput() {
	s.useKeyInput = true
	s.keyInput.SetValue("")
	s.keyInput.Focus()
}

func (s *providerStep) showCapabilities() {
	s.input.PromptMulti(
		i18n.T("wizard.provider.caps"),
		"capabilities", "",
		capabilityOptions(),
	)
	preselect := s.current.Capabilities
	if len(preselect) == 0 {
		preselect = defaultCapsForModel(s.current.Model)
	}
	if len(preselect) > 0 {
		s.input.PreSelectMulti(preselect)
	}
}

func (s *providerStep) showTags() {
	s.input.PromptText(
		i18n.T("wizard.provider.tags"),
		"tags", strings.Join(s.current.Tags, ", "), "", false, "",
	)
}

func (s *providerStep) showSystemPrompt() {
	s.input.PromptText(
		i18n.T("wizard.provider.prompt"),
		"system_prompt", s.current.SystemPrompt, "", false, "",
	)
}

func (s *providerStep) showAddMore() {
	s.input.PromptConfirm(i18n.T("wizard.provider.add_more"), "")
}

// --- Navigation helpers ---

func (s *providerStep) needsBaseURL() bool {
	if s.current.Driver == "ollama" || s.current.Driver == "openai-like" {
		return true
	}
	return s.customModel && driversWithBaseURL[s.current.Driver]
}

func (s *providerStep) needsAPIKey() bool {
	return s.current.Driver != "ollama" && s.current.Driver != "openai-like"
}

// isPhaseApplicable returns true if the phase should be visited given current state.
func (s *providerStep) isPhaseApplicable(p int) bool {
	switch p {
	case phaseCustomModel:
		return s.customModel
	case phaseBaseURL:
		return s.needsBaseURL()
	case phaseAPIKeyAsk:
		return s.needsAPIKey()
	case phaseAPIKeyInput:
		return false // only entered from APIKeyAsk "now"; skip in generic navigation
	default:
		return true
	}
}

// nextApplicablePhase returns the next applicable phase after from.
func (s *providerStep) nextApplicablePhase(from int) int {
	for p := from + 1; p <= phaseSystemPrompt; p++ {
		if s.isPhaseApplicable(p) {
			return p
		}
	}
	return phaseAddMore
}

// prevApplicablePhase returns the previous applicable phase before from.
func (s *providerStep) prevApplicablePhase(from int) int {
	for p := from - 1; p >= phaseDriver; p-- {
		if s.isPhaseApplicable(p) {
			return p
		}
	}
	return phaseDriver
}

func (s *providerStep) defaultAlias() string {
	base := driverProviderNames[s.current.Driver]
	if base == "" {
		base = s.current.Driver
	}
	if !s.aliasExists(base) {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !s.aliasExists(candidate) {
			return candidate
		}
	}
}

// resolveEnvVarName returns a unique env var name for the current provider.
// If the base env var (e.g. GOOGLE_API_KEY) is already used by another provider,
// it appends _ALIAS (uppercased, hyphens replaced) to make it unique.
func (s *providerStep) resolveEnvVarName() string {
	base := driverEnvVars[s.current.Driver]
	if base == "" {
		return ""
	}
	if !s.envVarUsed(base) {
		return base
	}
	suffix := strings.ToUpper(strings.ReplaceAll(s.current.Alias, "-", "_"))
	return base + "_" + suffix
}

// findReusableKey returns the first existing provider with the same driver
// that already has an API key configured. Returns nil if none found.
func (s *providerStep) findReusableKey() *ProviderEntry {
	for i := range s.providers {
		if s.providers[i].Driver == s.current.Driver && !s.providers[i].SkipKey && s.providers[i].EnvVarName != "" {
			return &s.providers[i]
		}
	}
	return nil
}

func (s *providerStep) envVarUsed(name string) bool {
	for _, p := range s.providers {
		if p.EnvVarName == name {
			return true
		}
	}
	return false
}

func (s *providerStep) aliasExists(alias string) bool {
	for _, p := range s.providers {
		if p.Alias == alias {
			return true
		}
	}
	return false
}

// --- Update ---

func (s *providerStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	// Handle esc.
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "esc" {
		return s.handleEsc()
	}

	// Password input mode.
	if s.useKeyInput {
		return s.updateKeyInput(msg)
	}

	// Handle InputResult from InputZone.
	if result, ok := msg.(components.InputResult); ok {
		return s.handleResult(result)
	}

	input, cmd := s.input.Update(msg)
	s.input = input
	return s, cmd
}

func (s *providerStep) handleEsc() (Step, tea.Cmd) {
	switch s.phase {
	case phaseDriver:
		if len(s.providers) > 0 {
			s.phase = phaseAddMore
			s.showPhase()
			return s, nil
		}
		return s, func() tea.Msg { return StepBackMsg{} }
	case phaseAddMore:
		s.phase = phaseSystemPrompt
		s.showPhase()
		return s, nil
	case phaseAPIKeyInput:
		// Back to ask.
		s.phase = phaseAPIKeyAsk
		s.showPhase()
		return s, nil
	default:
		s.phase = s.prevApplicablePhase(s.phase)
		s.showPhase()
		return s, nil
	}
}

func (s *providerStep) updateKeyInput(msg tea.Msg) (Step, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "enter":
			val := strings.TrimSpace(s.keyInput.Value())
			if val == "" {
				return s, nil
			}
			s.current.APIKey = val
			s.phase = s.nextApplicablePhase(phaseAPIKeyInput)
			s.showPhase()
			return s, nil
		case "esc":
			s.phase = phaseAPIKeyAsk
			s.showPhase()
			return s, nil
		}
	}
	var cmd tea.Cmd
	s.keyInput, cmd = s.keyInput.Update(msg)
	return s, cmd
}

func (s *providerStep) handleResult(result components.InputResult) (Step, tea.Cmd) {
	switch s.phase {
	case phaseDriver:
		s.current.Driver = result.Selected
		s.phase = phaseAlias
		s.showPhase()

	case phaseAlias:
		text := result.Text
		if text == "" {
			text = s.defaultAlias()
		}
		s.current.Alias = text
		s.current.EnvVarName = s.resolveEnvVarName()
		s.phase = phaseModel
		s.showPhase()

	case phaseModel:
		if result.Selected == customModelValue {
			s.customModel = true
			s.phase = phaseCustomModel
			s.showPhase()
			return s, nil
		}
		s.customModel = false
		s.current.Model = result.Selected
		s.phase = s.nextApplicablePhase(phaseModel)
		s.showPhase()

	case phaseCustomModel:
		s.current.Model = result.Text
		s.phase = s.nextApplicablePhase(phaseCustomModel)
		s.showPhase()

	case phaseBaseURL:
		text := result.Text
		if text == "" && s.current.Driver == "ollama" {
			text = "http://localhost:11434"
		} else if text == "" && s.current.Driver == "openai-like" {
			text = "http://localhost:8080/v1"
		}
		s.current.BaseURL = text
		s.phase = s.nextApplicablePhase(phaseBaseURL)
		s.showPhase()

	case phaseAPIKeyAsk:
		switch result.Selected {
		case "reuse":
			// Share the env var from the existing provider.
			if existing := s.findReusableKey(); existing != nil {
				s.current.EnvVarName = existing.EnvVarName
				s.current.SkipKey = true
			}
			s.phase = s.nextApplicablePhase(phaseAPIKeyInput)
			s.showPhase()
			return s, nil
		case "later":
			s.current.SkipKey = true
			s.phase = s.nextApplicablePhase(phaseAPIKeyInput)
			s.showPhase()
			return s, nil
		default: // "now"
			s.phase = phaseAPIKeyInput
			s.showPhase()
			return s, s.keyInput.Focus()
		}

	case phaseCaps:
		s.current.Capabilities = result.MultiSelect
		s.phase = phaseTags
		s.showPhase()

	case phaseTags:
		s.current.Tags = parseTags(result.Text)
		s.phase = phaseSystemPrompt
		s.showPhase()

	case phaseSystemPrompt:
		s.current.SystemPrompt = result.Text
		s.phase = phaseAddMore
		s.showPhase()

	case phaseAddMore:
		if result.Confirmed {
			// Save current (if configured) and start a new one.
			if s.current.Driver != "" {
				s.providers = append(s.providers, s.current)
			}
			s.current = ProviderEntry{}
			s.customModel = false
			s.phase = phaseDriver
			s.showPhase()
		} else {
			// Save current (if configured) and finish.
			if s.current.Driver != "" {
				s.providers = append(s.providers, s.current)
			}
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
	}

	return s, nil
}

func (s *providerStep) View() string {
	if s.useKeyInput {
		var b strings.Builder
		// Show completed fields.
		b.WriteString(s.input.View()[:0]) // not needed, we build manually
		if len(s.providers) > 0 {
			names := make([]string, len(s.providers))
			for i, p := range s.providers {
				names[i] = p.Alias
			}
			b.WriteString(components.AnswerLabelStyle.Render("> " + i18n.T("wizard.field.configured") + ": "))
			b.WriteString(components.AnswerStyle.Render(strings.Join(names, ", ")))
			b.WriteString("\n")
		}
		b.WriteString(components.LabelStyle.Render(fmt.Sprintf(i18n.T("wizard.provider.enter_key"), s.current.EnvVarName)))
		b.WriteString("\n")
		b.WriteString(s.keyInput.View())
		b.WriteString("\n")
		b.WriteString(components.HintStyle.Render("  " + i18n.T("hint.submit_back")))
		return b.String()
	}
	return s.input.View()
}

func (s *providerStep) Collect() Answers {
	a := Answers{"providers": s.providers}
	// Auto-set default if only one provider.
	if len(s.providers) == 1 {
		a["default_provider"] = s.providers[0].Alias
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
