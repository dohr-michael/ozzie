package setup_wizard

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/dohr-michael/ozzie/internal/i18n"
	"github.com/dohr-michael/ozzie/internal/ui/components"
)

// Phase constants for embedding configuration.
const (
	embPhaseKeepExisting = iota // shown when pre-filled from existing config
	embPhaseEnable
	embPhaseDriver
	embPhaseModel
	embPhaseCustomModel
	embPhaseBaseURL
	embPhaseAPIKeyAsk
	embPhaseAPIKeyInput
	embPhaseDims
)

// embeddingDriverEnvVars maps embedding drivers to their default env var.
var embeddingDriverEnvVars = map[string]string{
	"openai":  "OPENAI_API_KEY",
	"mistral": "MISTRAL_API_KEY",
	"gemini":  "GOOGLE_API_KEY",
}

// embModelInfo holds metadata for a predefined embedding model.
type embModelInfo struct {
	Value   string
	Label   string
	DescKey string
	Dims    int
}

// embDriverModels maps driver → list of predefined embedding models.
var embDriverModels = map[string][]embModelInfo{
	"openai": {
		{"text-embedding-3-small", "text-embedding-3-small", "wizard.emb_model.oai_small.desc", 1536},
		{"text-embedding-3-large", "text-embedding-3-large", "wizard.emb_model.oai_large.desc", 3072},
		{"text-embedding-ada-002", "text-embedding-ada-002", "wizard.emb_model.oai_ada.desc", 1536},
	},
	"ollama": {
		{"nomic-embed-text", "nomic-embed-text", "wizard.emb_model.nomic.desc", 768},
		{"mxbai-embed-large", "mxbai-embed-large", "wizard.emb_model.mxbai.desc", 1024},
		{"all-minilm", "all-minilm", "wizard.emb_model.minilm.desc", 384},
	},
	"mistral": {
		{"mistral-embed", "mistral-embed", "wizard.emb_model.mistral.desc", 1024},
	},
	"gemini": {
		{"gemini-embedding-001", "gemini-embedding-001", "wizard.emb_model.gem001.desc", 768},
		{"text-embedding-004", "text-embedding-004", "wizard.emb_model.gem004.desc", 768},
	},
}

// embModelDims returns the default dimensions for a known embedding model, or 0.
func embModelDims(model string) int {
	for _, models := range embDriverModels {
		for _, m := range models {
			if m.Value == model {
				return m.Dims
			}
		}
	}
	return 0
}

// embDriverSupportsDims returns true if the driver supports dimension override.
func embDriverSupportsDims(driver string) bool {
	return driver == "openai" || driver == "gemini"
}

// embModelOptions returns InputOptions for a given embedding driver.
func embModelOptions(driver string) []components.InputOption {
	models := embDriverModels[driver]
	opts := make([]components.InputOption, 0, len(models)+1)
	for _, m := range models {
		opts = append(opts, components.InputOption{
			Value:       m.Value,
			Label:       m.Label,
			Description: i18n.T(m.DescKey),
		})
	}
	opts = append(opts, components.InputOption{
		Value:       customModelValue,
		Label:       i18n.T("wizard.embedding.custom"),
		Description: i18n.T("wizard.embedding.custom.desc"),
	})
	return opts
}

type embeddingStep struct {
	input       *components.InputZone
	keyInput    textinput.Model
	phase       int
	customModel bool
	useKeyInput bool

	entry   EmbeddingEntry
	answers Answers
}

func newEmbeddingStep() *embeddingStep {
	ti := textinput.New()
	ti.Prompt = "  "
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	ti.CharLimit = 200
	ti.SetWidth(60)
	ti.Placeholder = "Paste your API key..."

	return &embeddingStep{
		input: components.NewInputZone(),
		keyInput: ti,
	}
}

func (s *embeddingStep) ID() string    { return "embedding" }
func (s *embeddingStep) Title() string { return "Embedding" }

func (s *embeddingStep) ShouldSkip(_ Answers) bool { return false }

func (s *embeddingStep) Init(answers Answers) tea.Cmd {
	s.answers = answers
	s.entry = EmbeddingEntry{}
	s.customModel = false
	s.useKeyInput = false

	// Pre-filled embedding config — ask to keep or reconfigure.
	if emb := answers.Embedding(); emb != nil && emb.Enabled {
		s.entry = *emb
		s.phase = embPhaseKeepExisting
		s.showPhase()
		return nil
	}

	s.phase = embPhaseEnable
	s.showPhase()
	return nil
}

// --- Phase display ---

func (s *embeddingStep) showPhase() {
	s.input.ClearCompletedFields()
	s.useKeyInput = false

	switch s.phase {
	case embPhaseKeepExisting:
		s.showKeepExisting()
	case embPhaseEnable:
		s.showEnable()
	case embPhaseDriver:
		s.addProgress()
		s.showDriverSelect()
	case embPhaseModel:
		s.addProgress()
		s.showModelSelect()
	case embPhaseCustomModel:
		s.addProgress()
		s.showCustomModelInput()
	case embPhaseBaseURL:
		s.addProgress()
		s.showBaseURL()
	case embPhaseAPIKeyAsk:
		s.addProgress()
		s.showAPIKeyAsk()
	case embPhaseAPIKeyInput:
		s.addProgress()
		s.showAPIKeyInput()
	case embPhaseDims:
		s.addProgress()
		s.showDims()
	}
}

func (s *embeddingStep) addProgress() {
	if s.entry.Driver != "" && s.phase > embPhaseDriver {
		s.input.AddCompletedField(i18n.T("wizard.field.driver"), s.entry.Driver)
	}
	if s.entry.Model != "" && s.phase > embPhaseModel && s.phase > embPhaseCustomModel {
		s.input.AddCompletedField(i18n.T("wizard.field.model"), s.entry.Model)
	}
	if s.entry.BaseURL != "" && s.phase > embPhaseBaseURL {
		s.input.AddCompletedField(i18n.T("wizard.field.base_url"), s.entry.BaseURL)
	}
}

func (s *embeddingStep) showKeepExisting() {
	s.input.AddCompletedField(i18n.T("wizard.field.driver"), s.entry.Driver)
	s.input.AddCompletedField(i18n.T("wizard.field.model"), s.entry.Model)
	if s.entry.Dims > 0 {
		s.input.AddCompletedField(i18n.T("wizard.embedding.dims"), strconv.Itoa(s.entry.Dims))
	}
	s.input.PromptConfirm(i18n.T("wizard.embedding.keep"), "")
}

func (s *embeddingStep) showEnable() {
	s.input.PromptConfirm(i18n.T("wizard.embedding.enable"), i18n.T("wizard.embedding.enable.desc"))
}

func (s *embeddingStep) showDriverSelect() {
	s.input.PromptSelect(
		i18n.T("wizard.embedding.driver"),
		"emb_driver", "",
		[]components.InputOption{
			{Value: "openai", Label: "OpenAI", Description: i18n.T("wizard.driver.openai_emb.desc")},
			{Value: "ollama", Label: "Ollama", Description: i18n.T("wizard.driver.ollama_emb.desc")},
			{Value: "mistral", Label: "Mistral", Description: i18n.T("wizard.driver.mistral_emb.desc")},
			{Value: "gemini", Label: "Google Gemini", Description: i18n.T("wizard.driver.gemini_emb.desc")},
		},
		"",
	)
}

func (s *embeddingStep) showModelSelect() {
	opts := embModelOptions(s.entry.Driver)
	s.input.PromptSelect(i18n.T("wizard.embedding.model"), "emb_model", "", opts, "")
}

func (s *embeddingStep) showCustomModelInput() {
	s.input.PromptText(i18n.T("wizard.embedding.model_id"), "emb_model", "", "", true, "")
}

func (s *embeddingStep) showBaseURL() {
	label := i18n.T("wizard.embedding.base_url")
	placeholder := ""
	if s.entry.Driver == "ollama" {
		label = i18n.T("wizard.embedding.base_url.ollama")
		placeholder = "http://localhost:11434"
	}
	s.input.PromptText(label, "emb_base_url", placeholder, "", false, "")
}

func (s *embeddingStep) showAPIKeyAsk() {
	envVar := s.entry.EnvVarName

	opts := []components.InputOption{
		{Value: "now", Label: i18n.T("wizard.embedding.key_new"), Description: fmt.Sprintf(i18n.T("wizard.embedding.key_new.desc"), envVar)},
		{Value: "later", Label: i18n.T("wizard.embedding.key_later"), Description: fmt.Sprintf(i18n.T("wizard.embedding.key_later.desc"), envVar)},
	}
	defaultOpt := "now"

	// Check if a LLM provider with the same driver already has a key.
	if reuse := s.findReusableLLMKey(); reuse != nil {
		opts = []components.InputOption{
			{Value: "reuse", Label: fmt.Sprintf(i18n.T("wizard.embedding.key_reuse"), reuse.EnvVarName), Description: i18n.T("wizard.embedding.key_reuse.desc")},
			{Value: "now", Label: i18n.T("wizard.embedding.key_new"), Description: fmt.Sprintf(i18n.T("wizard.embedding.key_new.desc"), envVar)},
			{Value: "later", Label: i18n.T("wizard.embedding.key_later"), Description: fmt.Sprintf(i18n.T("wizard.embedding.key_later.desc"), envVar)},
		}
		defaultOpt = "reuse"
	}

	s.input.PromptSelect(fmt.Sprintf(i18n.T("wizard.embedding.key_for"), envVar), "emb_key", "", opts, defaultOpt)
}

func (s *embeddingStep) showAPIKeyInput() {
	s.useKeyInput = true
	s.keyInput.SetValue("")
	s.keyInput.Focus()
}

func (s *embeddingStep) showDims() {
	defaultDims := embModelDims(s.entry.Model)
	placeholder := ""
	if defaultDims > 0 {
		placeholder = strconv.Itoa(defaultDims)
	}
	s.input.PromptText(i18n.T("wizard.embedding.dims"), "emb_dims", placeholder, "", false, `^\d*$`)
}

// --- Navigation ---

func (s *embeddingStep) needsBaseURL() bool {
	return s.entry.Driver == "ollama"
}

func (s *embeddingStep) needsAPIKey() bool {
	return s.entry.Driver != "ollama"
}

func (s *embeddingStep) needsDims() bool {
	return embDriverSupportsDims(s.entry.Driver)
}

func (s *embeddingStep) isPhaseApplicable(p int) bool {
	switch p {
	case embPhaseKeepExisting:
		return false // only entered from Init when pre-filled
	case embPhaseCustomModel:
		return s.customModel
	case embPhaseBaseURL:
		return s.needsBaseURL()
	case embPhaseAPIKeyAsk:
		return s.needsAPIKey()
	case embPhaseAPIKeyInput:
		return false // only entered from APIKeyAsk
	case embPhaseDims:
		return s.needsDims()
	default:
		return true
	}
}

func (s *embeddingStep) nextApplicablePhase(from int) int {
	for p := from + 1; p <= embPhaseDims; p++ {
		if s.isPhaseApplicable(p) {
			return p
		}
	}
	// Past last phase → done
	return -1
}

func (s *embeddingStep) prevApplicablePhase(from int) int {
	for p := from - 1; p >= embPhaseDriver; p-- {
		if s.isPhaseApplicable(p) {
			return p
		}
	}
	return embPhaseEnable
}

// findReusableLLMKey checks if a LLM provider with the same driver has an env var configured.
func (s *embeddingStep) findReusableLLMKey() *ProviderEntry {
	providers := s.answers.Providers()
	for i := range providers {
		if providers[i].Driver == s.entry.Driver && !providers[i].SkipKey && providers[i].EnvVarName != "" {
			return &providers[i]
		}
	}
	return nil
}

// resolveEmbEnvVarName returns the env var name for the embedding key.
func (s *embeddingStep) resolveEmbEnvVarName() string {
	base := embeddingDriverEnvVars[s.entry.Driver]
	if base == "" {
		return ""
	}
	// If a LLM provider already uses this env var, use a dedicated one.
	providers := s.answers.Providers()
	for _, p := range providers {
		if p.EnvVarName == base {
			// Use a dedicated embedding env var.
			driverUpper := strings.ToUpper(s.entry.Driver)
			return driverUpper + "_EMBEDDING_API_KEY"
		}
	}
	return base
}

// --- Update ---

func (s *embeddingStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "esc" {
		return s.handleEsc()
	}

	if s.useKeyInput {
		return s.updateKeyInput(msg)
	}

	if result, ok := msg.(components.InputResult); ok {
		return s.handleResult(result)
	}

	input, cmd := s.input.Update(msg)
	s.input = input
	return s, cmd
}

func (s *embeddingStep) handleEsc() (Step, tea.Cmd) {
	switch s.phase {
	case embPhaseKeepExisting:
		return s, func() tea.Msg { return StepBackMsg{} }
	case embPhaseEnable:
		return s, func() tea.Msg { return StepBackMsg{} }
	case embPhaseAPIKeyInput:
		s.phase = embPhaseAPIKeyAsk
		s.showPhase()
		return s, nil
	default:
		s.phase = s.prevApplicablePhase(s.phase)
		s.showPhase()
		return s, nil
	}
}

func (s *embeddingStep) updateKeyInput(msg tea.Msg) (Step, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "enter":
			val := strings.TrimSpace(s.keyInput.Value())
			if val == "" {
				return s, nil
			}
			s.entry.APIKey = val
			next := s.nextApplicablePhase(embPhaseAPIKeyInput)
			if next < 0 {
				return s, func() tea.Msg { return StepDoneMsg{} }
			}
			s.phase = next
			s.showPhase()
			return s, nil
		case "esc":
			s.phase = embPhaseAPIKeyAsk
			s.showPhase()
			return s, nil
		}
	}
	var cmd tea.Cmd
	s.keyInput, cmd = s.keyInput.Update(msg)
	return s, cmd
}

func (s *embeddingStep) handleResult(result components.InputResult) (Step, tea.Cmd) {
	switch s.phase {
	case embPhaseKeepExisting:
		if result.Confirmed {
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		// Reconfigure from scratch.
		s.entry = EmbeddingEntry{}
		s.phase = embPhaseEnable
		s.showPhase()
		return s, nil

	case embPhaseEnable:
		if !result.Confirmed {
			s.entry.Enabled = false
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		s.entry.Enabled = true
		s.phase = embPhaseDriver
		s.showPhase()

	case embPhaseDriver:
		s.entry.Driver = result.Selected
		s.entry.EnvVarName = s.resolveEmbEnvVarName()
		s.phase = embPhaseModel
		s.showPhase()

	case embPhaseModel:
		if result.Selected == customModelValue {
			s.customModel = true
			s.phase = embPhaseCustomModel
			s.showPhase()
			return s, nil
		}
		s.customModel = false
		s.entry.Model = result.Selected
		s.entry.Dims = embModelDims(result.Selected)
		next := s.nextApplicablePhase(embPhaseModel)
		if next < 0 {
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		s.phase = next
		s.showPhase()

	case embPhaseCustomModel:
		s.entry.Model = result.Text
		next := s.nextApplicablePhase(embPhaseCustomModel)
		if next < 0 {
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		s.phase = next
		s.showPhase()

	case embPhaseBaseURL:
		text := result.Text
		if text == "" && s.entry.Driver == "ollama" {
			text = "http://localhost:11434"
		}
		s.entry.BaseURL = text
		next := s.nextApplicablePhase(embPhaseBaseURL)
		if next < 0 {
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		s.phase = next
		s.showPhase()

	case embPhaseAPIKeyAsk:
		switch result.Selected {
		case "reuse":
			if reuse := s.findReusableLLMKey(); reuse != nil {
				s.entry.EnvVarName = reuse.EnvVarName
				s.entry.SkipKey = true
			}
			next := s.nextApplicablePhase(embPhaseAPIKeyInput)
			if next < 0 {
				return s, func() tea.Msg { return StepDoneMsg{} }
			}
			s.phase = next
			s.showPhase()
			return s, nil
		case "later":
			s.entry.SkipKey = true
			next := s.nextApplicablePhase(embPhaseAPIKeyInput)
			if next < 0 {
				return s, func() tea.Msg { return StepDoneMsg{} }
			}
			s.phase = next
			s.showPhase()
			return s, nil
		default: // "now"
			s.phase = embPhaseAPIKeyInput
			s.showPhase()
			return s, s.keyInput.Focus()
		}

	case embPhaseDims:
		text := result.Text
		if text == "" {
			text = strconv.Itoa(embModelDims(s.entry.Model))
		}
		dims, err := strconv.Atoi(text)
		if err != nil {
			dims = embModelDims(s.entry.Model)
		}
		s.entry.Dims = dims
		return s, func() tea.Msg { return StepDoneMsg{} }
	}

	return s, nil
}

func (s *embeddingStep) View() string {
	if s.useKeyInput {
		var b strings.Builder
		b.WriteString(components.LabelStyle.Render(fmt.Sprintf(i18n.T("wizard.embedding.enter_key"), s.entry.EnvVarName)))
		b.WriteString("\n")
		b.WriteString(s.keyInput.View())
		b.WriteString("\n")
		b.WriteString(components.HintStyle.Render("  " + i18n.T("hint.submit_back")))
		return b.String()
	}
	return s.input.View()
}

func (s *embeddingStep) Collect() Answers {
	if !s.entry.Enabled {
		return Answers{"embedding": (*EmbeddingEntry)(nil)}
	}
	e := s.entry // copy
	return Answers{"embedding": &e}
}
