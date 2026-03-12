package setup_wizard

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/dohr-michael/ozzie/internal/infra/i18n"
	"github.com/dohr-michael/ozzie/internal/infra/ui/components"
)

// inputAreaLines is the number of lines reserved for the confirm input + spacing.
const inputAreaLines = 4

type confirmStep struct {
	input     *components.InputZone
	vp        viewport.Model
	answers   Answers
	confirmed bool
	width     int
	height    int
	ready     bool
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

// SetSize implements the Resizable interface.
func (s *confirmStep) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.updateViewport()
}

func (s *confirmStep) Init(answers Answers) tea.Cmd {
	s.answers = answers
	s.confirmed = false
	s.input.ClearCompletedFields()
	s.input.PromptConfirm(i18n.T("wizard.confirm.apply"), "")
	s.vp.SetContent(s.buildSummary())
	s.vp.GotoTop()
	return nil
}

func (s *confirmStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "esc" {
		return s, func() tea.Msg { return StepBackMsg{} }
	}

	if result, ok := msg.(components.InputResult); ok {
		if result.Confirmed {
			s.confirmed = true
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		// "No" → go back.
		return s, func() tea.Msg { return StepBackMsg{} }
	}

	// Let viewport handle scroll keys (↑/↓/pgup/pgdn).
	var vpCmd tea.Cmd
	s.vp, vpCmd = s.vp.Update(msg)

	// Delegate to input as well.
	input, inputCmd := s.input.Update(msg)
	s.input = input

	return s, tea.Batch(vpCmd, inputCmd)
}

func (s *confirmStep) View() string {
	var b strings.Builder

	if !s.ready {
		// Viewport not yet sized — render summary directly.
		b.WriteString(s.buildSummary())
	} else {
		b.WriteString(s.vp.View())
		if s.vp.TotalLineCount() > s.vp.VisibleLineCount() {
			scrollHint := fmt.Sprintf("  %s  %.0f%%", i18n.T("hint.scroll"), s.vp.ScrollPercent()*100)
			b.WriteString("\n")
			b.WriteString(components.HintStyle.Render(scrollHint))
		}
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

// buildSummary renders the configuration summary text.
func (s *confirmStep) buildSummary() string {
	var b strings.Builder

	b.WriteString(components.LabelStyle.Render(i18n.T("wizard.confirm.title")))
	b.WriteString("\n\n")

	providers := s.answers.Providers()
	defaultProvider := s.answers.String("default_provider", "")
	host := s.answers.String("gateway_host", "127.0.0.1")
	port := s.answers.Int("gateway_port", 18420)

	// Providers summary.
	for _, p := range providers {
		isDefault := ""
		if p.Alias == defaultProvider {
			isDefault = i18n.T("wizard.confirm.default")
		}
		b.WriteString(formatRow(i18n.T("wizard.confirm.provider"), fmt.Sprintf("%s%s — %s (%s)", p.Alias, isDefault, p.Driver, p.Model)))

		if p.BaseURL != "" {
			b.WriteString(formatRow(i18n.T("wizard.confirm.base_url"), p.BaseURL))
		}

		if p.Driver == "ollama" {
			b.WriteString(formatRow(i18n.T("wizard.confirm.api_key"), i18n.T("wizard.confirm.key.none")))
		} else if p.APIKey != "" {
			b.WriteString(formatRow(i18n.T("wizard.confirm.api_key"), i18n.T("wizard.confirm.key.provided")))
		} else if p.SkipKey {
			b.WriteString(formatRow(i18n.T("wizard.confirm.api_key"), i18n.T("wizard.confirm.key.skipped")))
		}

		if len(p.Capabilities) > 0 {
			b.WriteString(formatRow(i18n.T("wizard.confirm.caps"), strings.Join(p.Capabilities, ", ")))
		}

		if len(p.Tags) > 0 {
			b.WriteString(formatRow(i18n.T("wizard.confirm.tags"), strings.Join(p.Tags, ", ")))
		}

		if p.SystemPrompt != "" {
			prompt := p.SystemPrompt
			if len(prompt) > 60 {
				prompt = prompt[:57] + "..."
			}
			b.WriteString(formatRow(i18n.T("wizard.confirm.prompt"), prompt))
		}

		b.WriteString("\n")
	}

	// Embedding.
	if emb := s.answers.Embedding(); emb != nil && emb.Enabled {
		b.WriteString(formatRow(i18n.T("wizard.confirm.embedding"), fmt.Sprintf("%s — %s (%d dims)", emb.Driver, emb.Model, emb.Dims)))
		if emb.SkipKey && emb.EnvVarName != "" {
			b.WriteString(formatRow(i18n.T("wizard.confirm.api_key"), fmt.Sprintf(i18n.T("wizard.confirm.emb_reuses"), emb.EnvVarName)))
		} else if emb.APIKey != "" {
			b.WriteString(formatRow(i18n.T("wizard.confirm.api_key"), i18n.T("wizard.confirm.key.provided")))
		} else if emb.Driver != "ollama" {
			b.WriteString(formatRow(i18n.T("wizard.confirm.api_key"), i18n.T("wizard.confirm.key.skipped")))
		}
		b.WriteString("\n")
	} else {
		b.WriteString(formatRow(i18n.T("wizard.confirm.embedding"), i18n.T("wizard.confirm.emb_disabled")))
		b.WriteString("\n")
	}

	// Layered context.
	if lc := s.answers.LayeredContext(); lc != nil && lc.Enabled {
		b.WriteString(formatRow(i18n.T("wizard.confirm.layered"), fmt.Sprintf("recent=%d, archives=%d", lc.MaxRecentMessages, lc.MaxArchives)))
		b.WriteString("\n")
	} else {
		b.WriteString(formatRow(i18n.T("wizard.confirm.layered"), i18n.T("wizard.confirm.layered_disabled")))
		b.WriteString("\n")
	}

	// MCP servers.
	if servers := s.answers.MCPServers(); len(servers) > 0 {
		for _, srv := range servers {
			label := fmt.Sprintf("%s — %s", srv.Name, srv.Transport)
			if srv.Transport == "stdio" {
				label += fmt.Sprintf(" (%s)", srv.Command)
			} else {
				label += fmt.Sprintf(" (%s)", srv.URL)
			}
			b.WriteString(formatRow(i18n.T("wizard.confirm.mcp"), label))
			if len(srv.TrustedTools) > 0 {
				b.WriteString(formatRow(i18n.T("wizard.mcp.field.trusted"), strings.Join(srv.TrustedTools, ", ")))
			}
		}
		b.WriteString("\n")
	} else {
		b.WriteString(formatRow(i18n.T("wizard.confirm.mcp"), i18n.T("wizard.confirm.mcp_none")))
		b.WriteString("\n")
	}

	// Gateway.
	b.WriteString(formatRow(i18n.T("wizard.confirm.gateway"), fmt.Sprintf("%s:%d", host, port)))

	return b.String()
}

// updateViewport recalculates viewport dimensions.
func (s *confirmStep) updateViewport() {
	if s.width == 0 || s.height == 0 {
		return
	}
	vpHeight := s.height - inputAreaLines
	if vpHeight < 3 {
		vpHeight = 3
	}
	s.vp.SetWidth(s.width)
	s.vp.SetHeight(vpHeight)
	s.ready = true
}

func formatRow(label, value string) string {
	l := components.AnswerLabelStyle.Render(fmt.Sprintf("  %-12s", label))
	v := components.AnswerStyle.Render(value)
	return l + v + "\n"
}
