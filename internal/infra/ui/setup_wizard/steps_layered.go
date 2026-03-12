package setup_wizard

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/dohr-michael/ozzie/internal/infra/i18n"
	"github.com/dohr-michael/ozzie/internal/infra/ui/components"
)

// Phase constants for layered context configuration.
const (
	layeredPhaseKeepExisting = iota
	layeredPhaseEnable
	layeredPhaseMaxRecent
	layeredPhaseMaxArchives
)

type layeredStep struct {
	input *components.InputZone
	phase int
	entry LayeredContextEntry
}

func newLayeredStep() *layeredStep {
	return &layeredStep{
		input: components.NewInputZone(),
	}
}

func (s *layeredStep) ID() string    { return "layered" }
func (s *layeredStep) Title() string { return "Layered Context" }

func (s *layeredStep) ShouldSkip(_ Answers) bool { return false }

func (s *layeredStep) Init(answers Answers) tea.Cmd {
	s.entry = LayeredContextEntry{}

	// Pre-filled layered context config — ask to keep or reconfigure.
	if lc := answers.LayeredContext(); lc != nil && lc.Enabled {
		s.entry = *lc
		s.phase = layeredPhaseKeepExisting
		s.showPhase()
		return nil
	}

	s.phase = layeredPhaseEnable
	s.showPhase()
	return nil
}

// --- Phase display ---

func (s *layeredStep) showPhase() {
	s.input.ClearCompletedFields()

	switch s.phase {
	case layeredPhaseKeepExisting:
		s.showKeepExisting()
	case layeredPhaseEnable:
		s.showEnable()
	case layeredPhaseMaxRecent:
		s.addProgress()
		s.showMaxRecent()
	case layeredPhaseMaxArchives:
		s.addProgress()
		s.showMaxArchives()
	}
}

func (s *layeredStep) addProgress() {
	if s.phase > layeredPhaseMaxRecent {
		s.input.AddCompletedField(
			i18n.T("wizard.layered.max_recent"),
			strconv.Itoa(s.entry.MaxRecentMessages),
		)
	}
}

func (s *layeredStep) showKeepExisting() {
	s.input.AddCompletedField(
		i18n.T("wizard.layered.max_recent"),
		strconv.Itoa(s.entry.MaxRecentMessages),
	)
	s.input.AddCompletedField(
		i18n.T("wizard.layered.max_archives"),
		strconv.Itoa(s.entry.MaxArchives),
	)
	s.input.PromptConfirm(i18n.T("wizard.layered.keep"), "")
}

func (s *layeredStep) showEnable() {
	s.input.PromptConfirm(i18n.T("wizard.layered.enable"), i18n.T("wizard.layered.enable.desc"))
}

func (s *layeredStep) showMaxRecent() {
	s.input.PromptText(
		i18n.T("wizard.layered.max_recent"),
		"layered_max_recent", "24",
		i18n.T("wizard.layered.max_recent.desc"),
		false, `^\d+$`,
	)
}

func (s *layeredStep) showMaxArchives() {
	s.input.PromptText(
		i18n.T("wizard.layered.max_archives"),
		"layered_max_archives", "12",
		i18n.T("wizard.layered.max_archives.desc"),
		false, `^\d+$`,
	)
}

// --- Update ---

func (s *layeredStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "esc" {
		return s.handleEsc()
	}

	if result, ok := msg.(components.InputResult); ok {
		return s.handleResult(result)
	}

	input, cmd := s.input.Update(msg)
	s.input = input
	return s, cmd
}

func (s *layeredStep) handleEsc() (Step, tea.Cmd) {
	switch s.phase {
	case layeredPhaseKeepExisting, layeredPhaseEnable:
		return s, func() tea.Msg { return StepBackMsg{} }
	case layeredPhaseMaxRecent:
		s.phase = layeredPhaseEnable
		s.showPhase()
		return s, nil
	case layeredPhaseMaxArchives:
		s.phase = layeredPhaseMaxRecent
		s.showPhase()
		return s, nil
	}
	return s, nil
}

func (s *layeredStep) handleResult(result components.InputResult) (Step, tea.Cmd) {
	switch s.phase {
	case layeredPhaseKeepExisting:
		if result.Confirmed {
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		// Reconfigure from scratch.
		s.entry = LayeredContextEntry{}
		s.phase = layeredPhaseEnable
		s.showPhase()
		return s, nil

	case layeredPhaseEnable:
		if !result.Confirmed {
			s.entry.Enabled = false
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		s.entry.Enabled = true
		s.phase = layeredPhaseMaxRecent
		s.showPhase()

	case layeredPhaseMaxRecent:
		text := result.Text
		if text == "" {
			text = "24"
		}
		n, err := strconv.Atoi(text)
		if err != nil {
			n = 24
		}
		s.entry.MaxRecentMessages = n
		s.phase = layeredPhaseMaxArchives
		s.showPhase()

	case layeredPhaseMaxArchives:
		text := result.Text
		if text == "" {
			text = "12"
		}
		n, err := strconv.Atoi(text)
		if err != nil {
			n = 12
		}
		s.entry.MaxArchives = n
		return s, func() tea.Msg { return StepDoneMsg{} }
	}

	return s, nil
}

func (s *layeredStep) View() string {
	return s.input.View()
}

func (s *layeredStep) Collect() Answers {
	if !s.entry.Enabled {
		return Answers{"layered_context": (*LayeredContextEntry)(nil)}
	}
	e := s.entry // copy
	return Answers{"layered_context": &e}
}
