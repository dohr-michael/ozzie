package setup_wizard

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dohr-michael/ozzie/internal/infra/i18n"
	"github.com/dohr-michael/ozzie/internal/infra/ui/components"
)

// Phase constants for MCP server configuration.
const (
	mcpPhaseEnable = iota
	mcpPhaseKeepExisting
	mcpPhaseName
	mcpPhaseTransport
	mcpPhaseCommand      // stdio only
	mcpPhaseArgs         // stdio only
	mcpPhaseURL          // sse/http only
	mcpPhaseEnvAsk
	mcpPhaseEnvName
	mcpPhaseEnvValue
	mcpPhaseEnvSecret    // only if value non-empty
	mcpPhaseProbe
	mcpPhaseConnecting   // waiting state
	mcpPhaseTrustedTools // multi-select
	mcpPhaseAddMore
)

// mcpProbeResultMsg is sent when the MCP probe completes.
type mcpProbeResultMsg struct {
	tools []string
	err   error
}

type mcpStep struct {
	input           *components.InputZone
	phase           int
	servers         []MCPServerEntry
	current         MCPServerEntry
	envVars         []MCPEnvVar
	currentEnv      MCPEnvVar
	discoveredTools []string
}

func newMCPStep() *mcpStep {
	return &mcpStep{
		input: components.NewInputZone(),
	}
}

func (s *mcpStep) ID() string    { return "mcp" }
func (s *mcpStep) Title() string { return "MCP Servers" }

func (s *mcpStep) ShouldSkip(_ Answers) bool { return false }

func (s *mcpStep) Init(answers Answers) tea.Cmd {
	s.servers = nil
	s.current = MCPServerEntry{}
	s.envVars = nil
	s.currentEnv = MCPEnvVar{}
	s.discoveredTools = nil

	// Pre-filled MCP servers — ask to keep or reconfigure.
	if existing := answers.MCPServers(); len(existing) > 0 {
		s.servers = existing
		s.phase = mcpPhaseKeepExisting
		s.showPhase()
		return nil
	}

	s.phase = mcpPhaseEnable
	s.showPhase()
	return nil
}

// --- Phase display ---

func (s *mcpStep) showPhase() {
	s.input.ClearCompletedFields()

	switch s.phase {
	case mcpPhaseEnable:
		s.showEnable()
	case mcpPhaseKeepExisting:
		s.showKeepExisting()
	case mcpPhaseName:
		s.addConfiguredProgress()
		s.showName()
	case mcpPhaseTransport:
		s.addConfiguredProgress()
		s.addCurrentProgress()
		s.showTransport()
	case mcpPhaseCommand:
		s.addConfiguredProgress()
		s.addCurrentProgress()
		s.showCommand()
	case mcpPhaseArgs:
		s.addConfiguredProgress()
		s.addCurrentProgress()
		s.showArgs()
	case mcpPhaseURL:
		s.addConfiguredProgress()
		s.addCurrentProgress()
		s.showURL()
	case mcpPhaseEnvAsk:
		s.addConfiguredProgress()
		s.addCurrentProgress()
		s.showEnvAsk()
	case mcpPhaseEnvName:
		s.addConfiguredProgress()
		s.addCurrentProgress()
		s.showEnvName()
	case mcpPhaseEnvValue:
		s.addConfiguredProgress()
		s.addCurrentProgress()
		s.showEnvValue()
	case mcpPhaseEnvSecret:
		s.addConfiguredProgress()
		s.addCurrentProgress()
		s.showEnvSecret()
	case mcpPhaseProbe:
		s.addConfiguredProgress()
		s.addCurrentProgress()
		s.showProbe()
	case mcpPhaseConnecting:
		s.addConfiguredProgress()
		s.addCurrentProgress()
		// Connecting state — no input needed.
	case mcpPhaseTrustedTools:
		s.addConfiguredProgress()
		s.addCurrentProgress()
		s.showTrustedTools()
	case mcpPhaseAddMore:
		s.addConfiguredProgress()
		s.showAddMore()
	}
}

func (s *mcpStep) addConfiguredProgress() {
	if len(s.servers) > 0 {
		names := make([]string, len(s.servers))
		for i, srv := range s.servers {
			names[i] = srv.Name
		}
		s.input.AddCompletedField(
			i18n.T("wizard.field.configured"),
			strings.Join(names, ", "),
		)
	}
}

func (s *mcpStep) addCurrentProgress() {
	if s.current.Name != "" && s.phase > mcpPhaseName {
		s.input.AddCompletedField(i18n.T("wizard.mcp.field.name"), s.current.Name)
	}
	if s.current.Transport != "" && s.phase > mcpPhaseTransport {
		s.input.AddCompletedField(i18n.T("wizard.mcp.field.transport"), s.current.Transport)
	}
	if s.current.Command != "" && s.phase > mcpPhaseCommand {
		s.input.AddCompletedField(i18n.T("wizard.mcp.field.command"), s.current.Command)
	}
	if len(s.current.Args) > 0 && s.phase > mcpPhaseArgs {
		s.input.AddCompletedField(i18n.T("wizard.mcp.field.args"), strings.Join(s.current.Args, " "))
	}
	if s.current.URL != "" && s.phase > mcpPhaseURL {
		s.input.AddCompletedField(i18n.T("wizard.mcp.field.url"), s.current.URL)
	}
	if len(s.envVars) > 0 && s.phase >= mcpPhaseEnvAsk {
		names := make([]string, len(s.envVars))
		for i, ev := range s.envVars {
			names[i] = ev.Name
		}
		s.input.AddCompletedField(i18n.T("wizard.mcp.field.env_vars"), strings.Join(names, ", "))
	}
}

func (s *mcpStep) showEnable() {
	s.input.PromptConfirm(i18n.T("wizard.mcp.enable"), i18n.T("wizard.mcp.enable.desc"))
}

func (s *mcpStep) showKeepExisting() {
	for _, srv := range s.servers {
		label := fmt.Sprintf("%s (%s)", srv.Name, srv.Transport)
		s.input.AddCompletedField(i18n.T("wizard.confirm.mcp"), label)
	}
	s.input.PromptConfirm(i18n.T("wizard.mcp.keep"), "")
}

func (s *mcpStep) showName() {
	s.input.PromptText(
		i18n.T("wizard.mcp.name"),
		"mcp_name", "",
		i18n.T("wizard.mcp.name.desc"),
		true, `^[a-zA-Z0-9_-]+$`,
	)
}

func (s *mcpStep) showTransport() {
	s.input.PromptSelect(
		i18n.T("wizard.mcp.transport"),
		"mcp_transport", "",
		[]components.InputOption{
			{Value: "stdio", Label: "stdio", Description: i18n.T("wizard.mcp.transport.stdio.desc")},
			{Value: "sse", Label: "sse", Description: i18n.T("wizard.mcp.transport.sse.desc")},
			{Value: "http", Label: "http", Description: i18n.T("wizard.mcp.transport.http.desc")},
		},
		"stdio",
	)
}

func (s *mcpStep) showCommand() {
	s.input.PromptText(
		i18n.T("wizard.mcp.command"),
		"mcp_command", "",
		i18n.T("wizard.mcp.command.desc"),
		true, "",
	)
}

func (s *mcpStep) showArgs() {
	s.input.PromptText(
		i18n.T("wizard.mcp.args"),
		"mcp_args", "",
		i18n.T("wizard.mcp.args.desc"),
		false, "",
	)
}

func (s *mcpStep) showURL() {
	s.input.PromptText(
		i18n.T("wizard.mcp.url"),
		"mcp_url", "",
		"",
		true, "",
	)
}

func (s *mcpStep) showEnvAsk() {
	s.input.PromptConfirm(i18n.T("wizard.mcp.env_ask"), i18n.T("wizard.mcp.env_ask.desc"))
}

func (s *mcpStep) showEnvName() {
	s.input.PromptText(
		i18n.T("wizard.mcp.env_name"),
		"mcp_env_name", "",
		"",
		true, `^[A-Z_][A-Z0-9_]*$`,
	)
}

func (s *mcpStep) showEnvValue() {
	s.input.PromptText(
		i18n.T("wizard.mcp.env_value"),
		"mcp_env_value", "",
		i18n.T("wizard.mcp.env_value.desc"),
		false, "",
	)
}

func (s *mcpStep) showEnvSecret() {
	s.input.PromptConfirm(i18n.T("wizard.mcp.env_secret"), "")
}

func (s *mcpStep) showProbe() {
	s.input.PromptConfirm(i18n.T("wizard.mcp.probe"), i18n.T("wizard.mcp.probe.desc"))
}

func (s *mcpStep) showTrustedTools() {
	options := make([]components.InputOption, len(s.discoveredTools))
	for i, t := range s.discoveredTools {
		options[i] = components.InputOption{Value: t, Label: t}
	}
	s.input.PromptMulti(
		i18n.T("wizard.mcp.trusted_tools"),
		"mcp_trusted_tools", "",
		options,
	)
}

func (s *mcpStep) showAddMore() {
	s.input.PromptConfirm(i18n.T("wizard.mcp.add_more"), "")
}

// --- Update ---

func (s *mcpStep) Update(msg tea.Msg) (Step, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			return s.handleEsc()
		}

	case mcpProbeResultMsg:
		return s.handleProbeResult(msg)

	case components.InputResult:
		return s.handleResult(msg)
	}

	input, cmd := s.input.Update(msg)
	s.input = input
	return s, cmd
}

func (s *mcpStep) handleEsc() (Step, tea.Cmd) {
	switch s.phase {
	case mcpPhaseEnable, mcpPhaseKeepExisting:
		return s, func() tea.Msg { return StepBackMsg{} }
	case mcpPhaseName:
		if len(s.servers) == 0 {
			s.phase = mcpPhaseEnable
			s.showPhase()
			return s, nil
		}
		// If we already have servers, go back to addMore of the previous server.
		s.phase = mcpPhaseAddMore
		s.showPhase()
		return s, nil
	case mcpPhaseTransport:
		s.phase = mcpPhaseName
	case mcpPhaseCommand:
		s.phase = mcpPhaseTransport
	case mcpPhaseArgs:
		s.phase = mcpPhaseCommand
	case mcpPhaseURL:
		s.phase = mcpPhaseTransport
	case mcpPhaseEnvAsk:
		if s.current.Transport == "stdio" {
			s.phase = mcpPhaseArgs
		} else {
			s.phase = mcpPhaseURL
		}
	case mcpPhaseEnvName:
		s.phase = mcpPhaseEnvAsk
	case mcpPhaseEnvValue:
		s.phase = mcpPhaseEnvName
	case mcpPhaseEnvSecret:
		s.phase = mcpPhaseEnvValue
	case mcpPhaseProbe:
		s.phase = mcpPhaseEnvAsk
	case mcpPhaseTrustedTools:
		s.phase = mcpPhaseProbe
	case mcpPhaseAddMore:
		s.phase = mcpPhaseProbe
	default:
		return s, nil
	}
	s.showPhase()
	return s, nil
}

func (s *mcpStep) handleResult(result components.InputResult) (Step, tea.Cmd) {
	switch s.phase {
	case mcpPhaseEnable:
		if !result.Confirmed {
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		s.phase = mcpPhaseName
		s.showPhase()
		return s, nil

	case mcpPhaseKeepExisting:
		if result.Confirmed {
			return s, func() tea.Msg { return StepDoneMsg{} }
		}
		// Reconfigure from scratch.
		s.servers = nil
		s.phase = mcpPhaseName
		s.showPhase()
		return s, nil

	case mcpPhaseName:
		s.current = MCPServerEntry{Name: result.Text}
		s.envVars = nil
		s.discoveredTools = nil
		s.phase = mcpPhaseTransport
		s.showPhase()

	case mcpPhaseTransport:
		s.current.Transport = result.Selected
		switch result.Selected {
		case "stdio":
			s.phase = mcpPhaseCommand
		default:
			s.phase = mcpPhaseURL
		}
		s.showPhase()

	case mcpPhaseCommand:
		s.current.Command = result.Text
		s.phase = mcpPhaseArgs
		s.showPhase()

	case mcpPhaseArgs:
		if result.Text != "" {
			s.current.Args = strings.Fields(result.Text)
		} else {
			s.current.Args = nil
		}
		s.phase = mcpPhaseEnvAsk
		s.showPhase()

	case mcpPhaseURL:
		s.current.URL = result.Text
		s.phase = mcpPhaseEnvAsk
		s.showPhase()

	case mcpPhaseEnvAsk:
		if result.Confirmed {
			s.currentEnv = MCPEnvVar{}
			s.phase = mcpPhaseEnvName
			s.showPhase()
			return s, nil
		}
		// No more env vars — proceed to probe.
		s.current.EnvVars = s.envVars
		s.phase = mcpPhaseProbe
		s.showPhase()

	case mcpPhaseEnvName:
		s.currentEnv.Name = result.Text
		s.phase = mcpPhaseEnvValue
		s.showPhase()

	case mcpPhaseEnvValue:
		s.currentEnv.Value = result.Text
		if result.Text != "" {
			s.phase = mcpPhaseEnvSecret
			s.showPhase()
		} else {
			// No value — not a secret, save and loop.
			s.currentEnv.IsSecret = false
			s.envVars = append(s.envVars, s.currentEnv)
			s.currentEnv = MCPEnvVar{}
			s.phase = mcpPhaseEnvAsk
			s.showPhase()
		}

	case mcpPhaseEnvSecret:
		s.currentEnv.IsSecret = result.Confirmed
		s.envVars = append(s.envVars, s.currentEnv)
		s.currentEnv = MCPEnvVar{}
		s.phase = mcpPhaseEnvAsk
		s.showPhase()

	case mcpPhaseProbe:
		if !result.Confirmed {
			s.finalizeCurrent()
			s.phase = mcpPhaseAddMore
			s.showPhase()
			return s, nil
		}
		// Start probe.
		s.phase = mcpPhaseConnecting
		s.showPhase()
		return s, probeMCPServer(s.current, s.envVars)

	case mcpPhaseTrustedTools:
		s.current.TrustedTools = result.MultiSelect
		s.finalizeCurrent()
		s.phase = mcpPhaseAddMore
		s.showPhase()

	case mcpPhaseAddMore:
		if result.Confirmed {
			s.current = MCPServerEntry{}
			s.envVars = nil
			s.discoveredTools = nil
			s.phase = mcpPhaseName
			s.showPhase()
			return s, nil
		}
		return s, func() tea.Msg { return StepDoneMsg{} }
	}

	return s, nil
}

func (s *mcpStep) handleProbeResult(msg mcpProbeResultMsg) (Step, tea.Cmd) {
	if msg.err != nil || len(msg.tools) == 0 {
		// Probe failed or no tools — skip trusted selection.
		s.finalizeCurrent()
		s.phase = mcpPhaseAddMore
		s.showPhase()
		if msg.err != nil {
			// Show error as a completed field before the addMore prompt.
			s.input.AddCompletedField("⚠", fmt.Sprintf(i18n.T("wizard.mcp.probe_failed"), msg.err))
		}
		return s, nil
	}

	s.discoveredTools = msg.tools
	s.phase = mcpPhaseTrustedTools
	s.showPhase()
	return s, nil
}

func (s *mcpStep) finalizeCurrent() {
	s.current.EnvVars = s.envVars
	s.servers = append(s.servers, s.current)
}

func (s *mcpStep) View() string {
	if s.phase == mcpPhaseConnecting {
		var b strings.Builder
		b.WriteString(s.input.View())
		b.WriteString("\n")
		b.WriteString(components.HintStyle.Render(
			fmt.Sprintf("  %s", fmt.Sprintf(i18n.T("wizard.mcp.connecting"), s.current.Name)),
		))
		return b.String()
	}
	return s.input.View()
}

func (s *mcpStep) Collect() Answers {
	if len(s.servers) == 0 {
		return Answers{"mcp_servers": ([]MCPServerEntry)(nil)}
	}
	return Answers{"mcp_servers": s.servers}
}

// probeMCPServer launches an async probe to discover available tools.
func probeMCPServer(entry MCPServerEntry, envVars []MCPEnvVar) tea.Cmd {
	return func() tea.Msg {
		tools, err := probeMCPTools(entry, envVars)
		return mcpProbeResultMsg{tools: tools, err: err}
	}
}
