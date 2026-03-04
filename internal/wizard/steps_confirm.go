package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dohr-michael/ozzie/clients/tui/components"
)

type confirmStep struct {
	input     *components.InputZone
	answers   Answers
	confirmed bool
}

func newConfirmStep() *confirmStep {
	return &confirmStep{
		input:   components.NewInputZone(),
		answers: make(Answers),
	}
}

func (s *confirmStep) ID() string    { return "confirm" }
func (s *confirmStep) Title() string { return "Confirm" }

func (s *confirmStep) ShouldSkip(_ Answers) bool { return false }

func (s *confirmStep) Init(answers Answers) tea.Cmd {
	s.answers = answers
	s.confirmed = false
	s.input.ClearCompletedFields()
	s.input.PromptConfirm("Apply this configuration?", "")
	return nil
}

func (s *confirmStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		return s, func() tea.Msg { return StepBackMsg{} }
	}

	if result, ok := msg.(components.InputResult); ok {
		if result.Confirmed {
			s.confirmed = true
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		// "No" → go back to provider step.
		return s, func() tea.Msg { return StepBackMsg{} }
	}

	input, cmd := s.input.Update(msg)
	s.input = input
	return s, cmd
}

func (s *confirmStep) View() string {
	var b strings.Builder

	b.WriteString(components.LabelStyle.Render("  Configuration Summary"))
	b.WriteString("\n\n")

	driver := s.answers.String("driver", "anthropic")
	model := s.answers.String("model", "")
	host := s.answers.String("gateway_host", "127.0.0.1")
	port := s.answers.Int("gateway_port", 18420)
	hasKey := s.answers.String("api_key", "") != ""
	skipKey := s.answers.Bool("skip_key", false)

	// Provider
	b.WriteString(formatRow("Provider", fmt.Sprintf("%s (%s)", driver, model)))

	// API Key
	if driver == "ollama" {
		b.WriteString(formatRow("API Key", "not required"))
	} else if hasKey {
		b.WriteString(formatRow("API Key", "provided (will be encrypted)"))
	} else if skipKey {
		b.WriteString(formatRow("API Key", "skipped (set later)"))
	}

	// Gateway
	b.WriteString(formatRow("Gateway", fmt.Sprintf("%s:%d", host, port)))

	// Base URL (ollama)
	if baseURL := s.answers.String("base_url", ""); baseURL != "" {
		b.WriteString(formatRow("Base URL", baseURL))
	}

	// Capabilities
	if caps := s.answers.Strings("capabilities", nil); len(caps) > 0 {
		b.WriteString(formatRow("Capabilities", strings.Join(caps, ", ")))
	}

	// Tags
	if tags := s.answers.Strings("tags", nil); len(tags) > 0 {
		b.WriteString(formatRow("Tags", strings.Join(tags, ", ")))
	}

	b.WriteString("\n")
	b.WriteString(s.input.View())

	return b.String()
}

func (s *confirmStep) Collect() Answers {
	return Answers{
		"confirmed": s.confirmed,
	}
}

func formatRow(label, value string) string {
	l := components.AnswerLabelStyle.Render(fmt.Sprintf("  %-12s", label))
	v := components.AnswerStyle.Render(value)
	return l + v + "\n"
}
