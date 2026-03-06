package setup_wizard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dohr-michael/ozzie/internal/i18n"
	"github.com/dohr-michael/ozzie/internal/ui/components"
)

type defaultStep struct {
	input   *components.InputZone
	chosen  string
	answers Answers
}

func newDefaultStep() *defaultStep {
	return &defaultStep{
		input:   components.NewInputZone(),
		answers: make(Answers),
	}
}

func (s *defaultStep) ID() string    { return "default" }
func (s *defaultStep) Title() string { return "Default Provider" }

// ShouldSkip returns true when there's only one provider (auto-selected).
func (s *defaultStep) ShouldSkip(answers Answers) bool {
	return len(answers.Providers()) <= 1
}

func (s *defaultStep) Init(answers Answers) tea.Cmd {
	s.answers = answers
	providers := answers.Providers()
	s.chosen = answers.String("default_provider", "")

	opts := make([]components.InputOption, len(providers))
	for i, p := range providers {
		opts[i] = components.InputOption{
			Value:       p.Alias,
			Label:       p.Alias,
			Description: p.Driver + " / " + p.Model,
		}
	}

	s.input.ClearCompletedFields()
	s.input.PromptSelect(
		i18n.T("wizard.default.which"),
		"default_provider", "",
		opts,
		s.chosen,
	)
	return nil
}

func (s *defaultStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
		return s, func() tea.Msg { return StepBackMsg{} }
	}

	if result, ok := msg.(components.InputResult); ok {
		s.chosen = result.Selected
		return s, func() tea.Msg { return StepDoneMsg{} }
	}

	input, cmd := s.input.Update(msg)
	s.input = input
	return s, cmd
}

func (s *defaultStep) View() string {
	return s.input.View()
}

func (s *defaultStep) Collect() Answers {
	return Answers{"default_provider": s.chosen}
}
