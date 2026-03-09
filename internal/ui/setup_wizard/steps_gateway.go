package setup_wizard

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/dohr-michael/ozzie/internal/i18n"
	"github.com/dohr-michael/ozzie/internal/ui/components"
)

type gatewayStep struct {
	input   *components.InputZone
	phase   int // 0=host, 1=port
	host    string
	port    int
	answers Answers
}

func newGatewayStep() *gatewayStep {
	return &gatewayStep{
		input:   components.NewInputZone(),
		answers: make(Answers),
	}
}

func (s *gatewayStep) ID() string    { return "gateway" }
func (s *gatewayStep) Title() string { return "Gateway" }

func (s *gatewayStep) ShouldSkip(_ Answers) bool { return false }

func (s *gatewayStep) Init(answers Answers) tea.Cmd {
	s.phase = 0
	s.host = answers.String("gateway_host", "127.0.0.1")
	s.port = answers.Int("gateway_port", 18420)
	s.input.ClearCompletedFields()
	s.showHostInput()
	return nil
}

func (s *gatewayStep) showHostInput() {
	s.input.PromptText(
		i18n.T("wizard.gateway.host"),
		"gateway_host", s.host, "", false, "",
	)
}

func (s *gatewayStep) showPortInput() {
	s.input.AddCompletedField(i18n.T("wizard.gateway.field.host"), s.host)
	s.input.PromptText(
		i18n.T("wizard.gateway.port"),
		"gateway_port", strconv.Itoa(s.port), "", false, `^\d+$`,
	)
}

func (s *gatewayStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	// Handle back on esc.
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "esc" {
		switch s.phase {
		case 0:
			return s, func() tea.Msg { return StepBackMsg{} }
		case 1:
			s.phase = 0
			s.input.ClearCompletedFields()
			s.showHostInput()
			return s, nil
		}
	}

	// Handle InputResult.
	if result, ok := msg.(components.InputResult); ok {
		switch s.phase {
		case 0:
			text := result.Text
			if text == "" {
				text = "127.0.0.1"
			}
			s.host = text
			s.phase = 1
			s.showPortInput()
			return s, nil

		case 1:
			text := result.Text
			if text == "" {
				text = "18420"
			}
			port, err := strconv.Atoi(text)
			if err != nil {
				port = 18420
			}
			s.port = port
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
	}

	input, cmd := s.input.Update(msg)
	s.input = input
	return s, cmd
}

func (s *gatewayStep) View() string {
	return s.input.View()
}

func (s *gatewayStep) Collect() Answers {
	return Answers{
		"gateway_host": s.host,
		"gateway_port": s.port,
	}
}
