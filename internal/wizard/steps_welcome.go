package wizard

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dohr-michael/ozzie/clients/tui/components"
	"github.com/dohr-michael/ozzie/internal/config"
)

type welcomeStep struct {
	configExists bool
	input        *components.InputZone
	answers      Answers
	showSelect   bool // true when config exists and we need user choice
	autoAdvance  bool // true when no config, auto-advance after delay
}

func newWelcomeStep() *welcomeStep {
	return &welcomeStep{
		input:   components.NewInputZone(),
		answers: make(Answers),
	}
}

func (s *welcomeStep) ID() string    { return "welcome" }
func (s *welcomeStep) Title() string { return "Welcome" }

func (s *welcomeStep) ShouldSkip(_ Answers) bool { return false }

func (s *welcomeStep) Init(answers Answers) tea.Cmd {
	s.answers = make(Answers)
	_, err := os.Stat(config.ConfigPath())
	s.configExists = err == nil

	if s.configExists {
		s.showSelect = true
		s.answers["existing_config"] = true
		s.input.ClearCompletedFields()
		s.input.PromptSelect(
			"Existing config found. What would you like to do?",
			"action", "",
			[]components.InputOption{
				{Value: "edit", Label: "Load existing & modify", Description: "Pre-fill from current config"},
				{Value: "fresh", Label: "Start fresh", Description: "Overwrite with new config"},
				{Value: "cancel", Label: "Cancel", Description: "Exit without changes"},
			},
			"edit",
		)
		return nil
	}

	// No config — auto-advance after short delay or on enter.
	s.showSelect = false
	s.autoAdvance = true
	s.answers["existing_config"] = false
	s.answers["action"] = "fresh"
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return autoAdvanceMsg{}
	})
}

type autoAdvanceMsg struct{}

func (s *welcomeStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg.(type) {
	case autoAdvanceMsg:
		if s.autoAdvance {
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "enter" && s.autoAdvance {
			s.autoAdvance = false
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
	}

	if s.showSelect {
		input, cmd := s.input.Update(msg)
		s.input = input

		if result, ok := msg.(components.InputResult); ok {
			s.answers["action"] = result.Selected
			if result.Selected == "edit" {
				s.prefillFromConfig()
			}
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		return s, cmd
	}

	return s, nil
}

func (s *welcomeStep) View() string {
	if s.showSelect {
		return s.input.View()
	}

	title := lipgloss.NewStyle().
		Foreground(components.Secondary).
		Bold(true).
		Render("  Welcome to Ozzie!")

	subtitle := components.HintStyle.Render("  Let's set up your agent OS. Press enter to continue...")

	return title + "\n\n" + subtitle
}

func (s *welcomeStep) Collect() Answers {
	return s.answers
}

// prefillFromConfig loads the existing config and extracts values into answers.
func (s *welcomeStep) prefillFromConfig() {
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		return
	}

	if cfg.Models.Default != "" && cfg.Models.Providers != nil {
		provider, ok := cfg.Models.Providers[cfg.Models.Default]
		if ok {
			s.answers["driver"] = provider.Driver
			s.answers["model"] = provider.Model
			if provider.BaseURL != "" {
				s.answers["base_url"] = provider.BaseURL
			}
			if len(provider.Capabilities) > 0 {
				s.answers["capabilities"] = provider.Capabilities
			}
			if len(provider.Tags) > 0 {
				s.answers["tags"] = provider.Tags
			}
		}
	}

	if cfg.Gateway.Host != "" {
		s.answers["gateway_host"] = cfg.Gateway.Host
	}
	if cfg.Gateway.Port != 0 {
		s.answers["gateway_port"] = cfg.Gateway.Port
	}
}
