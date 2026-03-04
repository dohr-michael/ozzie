package wizard

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dohr-michael/ozzie/clients/tui/components"
)

type apiKeyStep struct {
	input      *components.InputZone
	keyInput   textinput.Model
	phase      int // 0=ask, 1=enter key
	skipKey    bool
	apiKey     string
	envVarName string
	driver     string
	useKeyInput bool // true when in password mode
}

func newAPIKeyStep() *apiKeyStep {
	ti := textinput.New()
	ti.Prompt = "  "
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	ti.CharLimit = 200
	ti.Width = 60
	ti.Placeholder = "Paste your API key..."

	return &apiKeyStep{
		input:    components.NewInputZone(),
		keyInput: ti,
	}
}

func (s *apiKeyStep) ID() string    { return "apikey" }
func (s *apiKeyStep) Title() string { return "API Key" }

func (s *apiKeyStep) ShouldSkip(answers Answers) bool {
	return answers.String("driver", "") == "ollama"
}

func (s *apiKeyStep) Init(answers Answers) tea.Cmd {
	s.phase = 0
	s.skipKey = false
	s.apiKey = ""
	s.driver = answers.String("driver", "anthropic")
	s.envVarName = driverEnvVars[s.driver]
	s.useKeyInput = false
	s.input.ClearCompletedFields()
	s.showAskPrompt()
	return nil
}

func (s *apiKeyStep) showAskPrompt() {
	s.useKeyInput = false
	s.input.PromptSelect(
		"API Key for "+s.envVarName,
		"ask_key", "",
		[]components.InputOption{
			{Value: "now", Label: "Enter API key now", Description: "Will be encrypted with age"},
			{Value: "later", Label: "I'll set it later", Description: "Use: ozzie secret set " + s.envVarName},
		},
		"now",
	)
}

func (s *apiKeyStep) showKeyInput() {
	s.useKeyInput = true
	s.keyInput.SetValue("")
	s.keyInput.Focus()
}

func (s *apiKeyStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	// Handle back on esc.
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		if s.phase == 1 {
			s.phase = 0
			s.showAskPrompt()
			return s, nil
		}
		return s, func() tea.Msg { return StepBackMsg{} }
	}

	if s.useKeyInput {
		// Password input mode.
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				val := strings.TrimSpace(s.keyInput.Value())
				if val == "" {
					return s, nil // Don't submit empty.
				}
				s.apiKey = val
				return s, func() tea.Msg { return StepDoneMsg{} }
			}
		}

		var cmd tea.Cmd
		s.keyInput, cmd = s.keyInput.Update(msg)
		return s, cmd
	}

	// Select mode.
	if result, ok := msg.(components.InputResult); ok {
		switch s.phase {
		case 0:
			if result.Selected == "later" {
				s.skipKey = true
				return s, func() tea.Msg { return StepDoneMsg{} }
			}
			s.phase = 1
			s.showKeyInput()
			return s, s.keyInput.Focus()
		}
	}

	input, cmd := s.input.Update(msg)
	s.input = input
	return s, cmd
}

func (s *apiKeyStep) View() string {
	if s.useKeyInput {
		var b strings.Builder
		b.WriteString(components.LabelStyle.Render("Enter your " + s.envVarName + ":"))
		b.WriteString("\n")
		b.WriteString(s.keyInput.View())
		b.WriteString("\n")
		b.WriteString(components.HintStyle.Render("  enter=submit • esc=back"))
		return b.String()
	}
	return s.input.View()
}

func (s *apiKeyStep) Collect() Answers {
	return Answers{
		"api_key":      s.apiKey,
		"env_var_name": s.envVarName,
		"skip_key":     s.skipKey,
	}
}
