package setup_wizard

import (
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/i18n"
	"github.com/dohr-michael/ozzie/internal/ui/components"
)

const (
	welcomePhaseLanguage = iota
	welcomePhaseAction
	welcomePhaseAutoAdvance
)

type welcomeStep struct {
	configExists bool
	input        *components.InputZone
	answers      Answers
	phase        int
	autoAdvance  bool
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

func (s *welcomeStep) Init(_ Answers) tea.Cmd {
	s.answers = make(Answers)
	_, err := os.Stat(config.ConfigPath())
	s.configExists = err == nil

	// Start with language selection.
	s.phase = welcomePhaseLanguage
	s.autoAdvance = false
	s.showLanguageSelect()
	return nil
}

func (s *welcomeStep) showLanguageSelect() {
	s.input.ClearCompletedFields()
	s.input.PromptSelect(
		i18n.T("wizard.welcome.language"),
		"language", "",
		[]components.InputOption{
			{Value: "en", Label: "English"},
			{Value: "fr", Label: "Français"},
		},
		i18n.Lang,
	)
}

func (s *welcomeStep) showActionSelect() {
	s.input.ClearCompletedFields()
	s.input.PromptSelect(
		i18n.T("wizard.welcome.existing_q"),
		"action", "",
		[]components.InputOption{
			{Value: "edit", Label: i18n.T("wizard.welcome.load"), Description: i18n.T("wizard.welcome.load.desc")},
			{Value: "fresh", Label: i18n.T("wizard.welcome.fresh"), Description: i18n.T("wizard.welcome.fresh.desc")},
			{Value: "cancel", Label: i18n.T("wizard.welcome.cancel"), Description: i18n.T("wizard.welcome.cancel.desc")},
		},
		"edit",
	)
}

type autoAdvanceMsg struct{}

func (s *welcomeStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg.(type) {
	case autoAdvanceMsg:
		if s.autoAdvance {
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if keyMsg.String() == "enter" && s.autoAdvance {
			s.autoAdvance = false
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
	}

	if s.phase == welcomePhaseLanguage || s.phase == welcomePhaseAction {
		input, cmd := s.input.Update(msg)
		s.input = input

		if result, ok := msg.(components.InputResult); ok {
			return s.handleResult(result)
		}
		return s, cmd
	}

	return s, nil
}

func (s *welcomeStep) handleResult(result components.InputResult) (Step, tea.Cmd) {
	switch s.phase {
	case welcomePhaseLanguage:
		i18n.Lang = result.Selected
		s.answers["preferred_language"] = result.Selected

		if s.configExists {
			s.phase = welcomePhaseAction
			s.answers["existing_config"] = true
			s.showActionSelect()
			return s, nil
		}

		// No config — auto-advance.
		s.phase = welcomePhaseAutoAdvance
		s.autoAdvance = true
		s.answers["existing_config"] = false
		s.answers["action"] = "fresh"
		return s, tea.Tick(time.Second, func(_ time.Time) tea.Msg {
			return autoAdvanceMsg{}
		})

	case welcomePhaseAction:
		s.answers["action"] = result.Selected
		if result.Selected == "edit" {
			s.prefillFromConfig()
		}
		return s, func() tea.Msg { return StepDoneMsg{} }
	}

	return s, nil
}

func (s *welcomeStep) View() string {
	if s.phase == welcomePhaseAutoAdvance {
		title := lipgloss.NewStyle().
			Foreground(components.Secondary).
			Bold(true).
			Render(i18n.T("wizard.welcome.title"))

		subtitle := components.HintStyle.Render(i18n.T("wizard.welcome.subtitle"))

		return title + "\n\n" + subtitle
	}

	return s.input.View()
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

	// Build provider entries from all configured providers.
	if cfg.Models.Providers != nil {
		var providers []ProviderEntry
		for alias, p := range cfg.Models.Providers {
			entry := ProviderEntry{
				Alias:        alias,
				Driver:       p.Driver,
				Model:        p.Model,
				BaseURL:      p.BaseURL,
				EnvVarName:   driverEnvVars[p.Driver],
				SkipKey:      true, // don't re-prompt for existing keys
				Capabilities: p.Capabilities,
				Tags:         p.Tags,
				SystemPrompt: p.PromptPrefix,
			}
			providers = append(providers, entry)
		}
		if len(providers) > 0 {
			s.answers["providers"] = providers
		}
	}

	if cfg.Models.Default != "" {
		s.answers["default_provider"] = cfg.Models.Default
	}

	if cfg.Embedding.IsEnabled() {
		entry := EmbeddingEntry{
			Enabled:    true,
			Driver:     cfg.Embedding.Driver,
			Model:      cfg.Embedding.Model,
			BaseURL:    cfg.Embedding.BaseURL,
			Dims:       cfg.Embedding.Dims,
			EnvVarName: embeddingDriverEnvVars[cfg.Embedding.Driver],
			SkipKey:    true, // don't re-prompt for existing keys
		}
		s.answers["embedding"] = &entry
	}

	if cfg.LayeredContext.IsEnabled() {
		entry := LayeredContextEntry{
			Enabled:           true,
			MaxRecentMessages: cfg.LayeredContext.MaxRecentMessages,
			MaxArchives:       cfg.LayeredContext.MaxArchives,
		}
		s.answers["layered_context"] = &entry
	}

	if len(cfg.MCP.Servers) > 0 {
		var servers []MCPServerEntry
		for name, srv := range cfg.MCP.Servers {
			entry := MCPServerEntry{
				Name:         name,
				Transport:    srv.Transport,
				Command:      srv.Command,
				Args:         srv.Args,
				URL:          srv.URL,
				TrustedTools: srv.TrustedTools,
			}
			for k := range srv.Env {
				entry.EnvVars = append(entry.EnvVars, MCPEnvVar{Name: k, IsSecret: false})
			}
			servers = append(servers, entry)
		}
		s.answers["mcp_servers"] = servers
	}

	if cfg.Gateway.Host != "" {
		s.answers["gateway_host"] = cfg.Gateway.Host
	}
	if cfg.Gateway.Port != 0 {
		s.answers["gateway_port"] = cfg.Gateway.Port
	}
}
