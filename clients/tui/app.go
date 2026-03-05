package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dohr-michael/ozzie/internal/ui/components"
	wsclient "github.com/dohr-michael/ozzie/clients/ws"
	"github.com/dohr-michael/ozzie/internal/events"
)

// App is the main TUI application model.
// Architecture: terminal-streaming with tea.Println for flushed history
// and a small active zone rendered in View().
type App struct {
	// Components (sticky footer)
	header    *components.Header
	inputZone *components.InputZone

	// Active interaction state (rendered in View)
	activeTools []components.ToolCall
	streaming   string
	showThinking bool

	// State
	width            int
	height           int
	isStreaming       bool
	quitting         bool

	// Current prompt state (token for response)
	currentPromptToken string

	// Dependencies
	client    *wsclient.Client
	sessionID string
}

// NewApp creates a new TUI application.
func NewApp(client *wsclient.Client, sessionID string) *App {
	return &App{
		header:    components.NewHeader(),
		inputZone: components.NewInputZone(),
		client:    client,
		sessionID: sessionID,
	}
}

// Init initializes the application.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		tea.ClearScreen,
		a.inputZone.Init(),
		a.inputZone.Focus(),
		tea.Println(components.RenderWelcome()),
	)
}

// Update handles messages and updates state.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updateSizes()
		if a.inputZone.Mode() == components.ModeChat && !a.isStreaming {
			cmds = append(cmds, a.inputZone.Focus())
		}

	case tea.KeyMsg:
		// Drop unparsed SGR mouse escape sequence fragments.
		if msg.Type == tea.KeyRunes && isMouseEscapeFragment(string(msg.Runes)) {
			return a, nil
		}

		switch msg.String() {
		case "ctrl+c":
			a.quitting = true
			return a, tea.Quit
		}

		// Update input zone
		var cmd tea.Cmd
		a.inputZone, cmd = a.inputZone.Update(msg)
		cmds = append(cmds, cmd)

	case components.InputResult:
		cmds = append(cmds, a.handleInputResult(msg))

	// --- Ozzie WS messages ---

	case StreamStartMsg:
		a.isStreaming = true
		a.streaming = ""
		a.showThinking = true
		a.header.SetStreaming(true)
		a.inputZone.SetDisabled(true)

	case StreamDeltaMsg:
		a.showThinking = false
		a.streaming += msg.Content

	case StreamEndMsg:
		// No-op: content finalized by AssistantMessageMsg

	case AssistantMessageMsg:
		// Flush any remaining active tools
		cmds = append(cmds, a.flushActiveTools()...)

		a.isStreaming = false
		a.showThinking = false
		a.header.SetStreaming(false)
		a.inputZone.SetDisabled(false)
		a.streaming = ""

		if msg.Error != "" {
			cmds = append(cmds, tea.Println("\n"+components.RenderError(msg.Error, a.width)))
		} else if msg.Content != "" {
			cmds = append(cmds, tea.Println("\n"+components.RenderAssistantMessage(msg.Content, a.width)))
		}
		cmds = append(cmds, a.inputZone.Focus())
		return a, tea.Batch(cmds...)

	case ToolCallMsg:
		cmds = append(cmds, a.handleToolCall(msg)...)

	case PromptRequestMsg:
		cmds = append(cmds, a.handlePromptRequest(msg)...)

	case LLMTelemetryMsg:
		a.header.AddTokens(msg.TokensOut)

	case ConnectedMsg:
		if msg.Client != nil {
			a.client = msg.Client
		}
		a.sessionID = msg.SessionID

	case DisconnectedMsg:
		// Phase 2

	case sendErrorMsg:
		cmds = append(cmds, tea.Println(components.RenderError(fmt.Sprintf("Send error: %v", msg.err), a.width)))
		return a, tea.Batch(cmds...)
	}

	return a, tea.Batch(cmds...)
}

// View renders only the active zone (small, constant cost).
func (a *App) View() string {
	if a.quitting {
		return ""
	}

	var parts []string

	// Active zone: in-progress tools + streaming text
	if active := a.renderActive(); active != "" {
		parts = append(parts, active)
	}

	parts = append(parts, a.inputZone.View(), a.header.View())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderActive renders only in-progress elements (tools + streaming + thinking).
func (a *App) renderActive() string {
	var parts []string

	// In-progress tool calls
	if len(a.activeTools) > 0 {
		parts = append(parts, components.RenderExpandedTools(a.activeTools, a.width))
	}

	// Thinking indicator (before any stream content arrives)
	if a.showThinking && a.streaming == "" {
		parts = append(parts, components.RenderThinking())
	}

	// Streaming text (raw, no glamour)
	if a.streaming != "" {
		parts = append(parts, components.RenderStreamingText(a.streaming))
	}

	if len(parts) == 0 {
		return ""
	}
	return "\n" + strings.Join(parts, "\n")
}

func (a *App) updateSizes() {
	a.header.SetWidth(a.width)
	a.inputZone.SetSize(a.width, a.inputHeightForMode(a.inputZone.Mode()))
}

// inputHeightForMode returns the appropriate input zone height for a given mode.
func (a *App) inputHeightForMode(mode components.InputMode) int {
	switch mode {
	case components.ModeChat:
		return 3
	case components.ModeConfirm:
		return 7
	case components.ModeText:
		return 7
	case components.ModeSelect, components.ModeMulti:
		return 10
	default:
		return 3
	}
}

// flushActiveTools flushes all completed active tools via tea.Println and clears the list.
func (a *App) flushActiveTools() []tea.Cmd {
	if len(a.activeTools) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, tool := range a.activeTools {
		cmds = append(cmds, tea.Println(components.RenderToolResult(tool, a.width)))
	}
	a.activeTools = nil
	return cmds
}

// handleInputResult processes input from InputZone and bridges to WS.
func (a *App) handleInputResult(result components.InputResult) tea.Cmd {
	if result.Cancelled {
		if a.currentPromptToken != "" {
			token := a.currentPromptToken
			a.currentPromptToken = ""
			a.inputZone.Reset()
			a.updateSizes()
			return tea.Batch(a.inputZone.Focus(), a.sendPromptCancel(token))
		}
		a.inputZone.Reset()
		a.updateSizes()
		return a.inputZone.Focus()
	}

	switch result.Mode {
	case components.ModeChat:
		text := result.Text
		if strings.HasPrefix(text, "/") {
			return a.handleSlashCommand(text)
		}

		// Flush user message to scrollback (with breathing room)
		printCmd := tea.Println("\n" + components.RenderUserMessage(text, a.width))

		a.inputZone.SetDisabled(true)
		a.showThinking = true
		a.isStreaming = true
		a.streaming = ""
		a.header.SetStreaming(true)

		client := a.client
		sendCmd := func() tea.Msg {
			if err := client.SendMessage(text); err != nil {
				return sendErrorMsg{err: err}
			}
			return nil
		}

		return tea.Batch(printCmd, sendCmd)

	case components.ModeConfirm:
		token := a.currentPromptToken
		if result.ResumeToken != "" {
			token = result.ResumeToken
		}
		a.currentPromptToken = ""
		a.updateSizes()

		if result.Confirmed {
			a.showThinking = true
			a.isStreaming = true
			a.header.SetStreaming(true)
		}

		client := a.client
		cancelled := !result.Confirmed
		return func() tea.Msg {
			if err := client.RespondToPrompt(token, cancelled); err != nil {
				return sendErrorMsg{err: err}
			}
			return nil
		}

	case components.ModeText:
		token := a.currentPromptToken
		a.currentPromptToken = ""
		a.inputZone.Reset()
		a.updateSizes()
		a.showThinking = true
		a.isStreaming = true
		a.header.SetStreaming(true)

		client := a.client
		value := result.Text
		return func() tea.Msg {
			if err := client.RespondToPromptWithValue(token, value); err != nil {
				return sendErrorMsg{err: err}
			}
			return nil
		}

	case components.ModeSelect:
		token := a.currentPromptToken
		a.currentPromptToken = ""
		a.inputZone.Reset()
		a.updateSizes()
		a.showThinking = true
		a.isStreaming = true
		a.header.SetStreaming(true)

		client := a.client
		value := result.Selected
		return func() tea.Msg {
			if err := client.RespondToPromptWithValue(token, value); err != nil {
				return sendErrorMsg{err: err}
			}
			return nil
		}

	case components.ModeMulti:
		token := a.currentPromptToken
		a.currentPromptToken = ""
		a.inputZone.Reset()
		a.updateSizes()
		a.showThinking = true
		a.isStreaming = true
		a.header.SetStreaming(true)

		client := a.client
		values := result.MultiSelect
		return func() tea.Msg {
			if err := client.RespondToPromptWithValues(token, values); err != nil {
				return sendErrorMsg{err: err}
			}
			return nil
		}
	}

	return nil
}

// handleToolCall dispatches tool call events.
func (a *App) handleToolCall(msg ToolCallMsg) []tea.Cmd {
	switch msg.Status {
	case string(events.ToolStatusStarted):
		args := components.FormatArguments(msg.Arguments)
		a.showThinking = false
		a.activeTools = append(a.activeTools, components.ToolCall{
			Name:      msg.Name,
			Arguments: args,
		})

	case string(events.ToolStatusCompleted):
		// Find matching active tool, mark completed, flush it
		for i := len(a.activeTools) - 1; i >= 0; i-- {
			if a.activeTools[i].Name == msg.Name && !a.activeTools[i].Completed {
				a.activeTools[i].Result = msg.Result
				a.activeTools[i].Status = components.ToolStatusCompleted
				a.activeTools[i].Completed = true

				// Flush this tool to scrollback
				printCmd := tea.Println(components.RenderToolResult(a.activeTools[i], a.width))
				// Remove from active list
				a.activeTools = append(a.activeTools[:i], a.activeTools[i+1:]...)
				return []tea.Cmd{printCmd}
			}
		}

	case string(events.ToolStatusFailed):
		for i := len(a.activeTools) - 1; i >= 0; i-- {
			if a.activeTools[i].Name == msg.Name && !a.activeTools[i].Completed {
				a.activeTools[i].Error = fmt.Errorf("%s", msg.Error)
				a.activeTools[i].Status = components.ToolStatusFailed
				a.activeTools[i].Completed = true

				printCmd := tea.Println(components.RenderToolResult(a.activeTools[i], a.width))
				a.activeTools = append(a.activeTools[:i], a.activeTools[i+1:]...)
				return []tea.Cmd{printCmd}
			}
		}
	}

	return nil
}

// handlePromptRequest bridges PromptRequestMsg to InputZone prompts.
func (a *App) handlePromptRequest(msg PromptRequestMsg) []tea.Cmd {
	// Flush active tools before showing prompt
	cmds := a.flushActiveTools()

	// Flush any streaming content
	if a.streaming != "" {
		cmds = append(cmds, tea.Println(components.RenderAssistantMessage(a.streaming, a.width)))
		a.streaming = ""
	}

	a.isStreaming = false
	a.showThinking = false
	a.header.SetStreaming(false)
	a.inputZone.SetDisabled(false)
	a.currentPromptToken = msg.Token

	switch msg.Type {
	case "confirm":
		a.inputZone.PromptConfirm(msg.Label, msg.Token)

	case "text", "password":
		placeholder := msg.Placeholder
		if placeholder == "" {
			placeholder = "Enter " + strings.ToLower(msg.Label) + "..."
		}
		a.inputZone.PromptText(msg.Label, "", placeholder, msg.Token, false, "")

	case "select":
		options := make([]components.InputOption, len(msg.Options))
		for i, opt := range msg.Options {
			options[i] = components.InputOption{
				Value:       opt.Value,
				Label:       opt.Label,
				Description: opt.Description,
			}
		}
		a.inputZone.PromptSelect(msg.Label, "", msg.Token, options, "")

	case "multi":
		options := make([]components.InputOption, len(msg.Options))
		for i, opt := range msg.Options {
			options[i] = components.InputOption{
				Value:       opt.Value,
				Label:       opt.Label,
				Description: opt.Description,
			}
		}
		a.inputZone.PromptMulti(msg.Label, "", msg.Token, options)
	}

	// Resize for the new input mode
	a.updateSizes()
	return cmds
}

// sendPromptCancel sends a cancellation response to the gateway.
func (a *App) sendPromptCancel(token string) tea.Cmd {
	client := a.client
	return func() tea.Msg {
		if err := client.RespondToPrompt(token, true); err != nil {
			return sendErrorMsg{err: err}
		}
		return nil
	}
}

// handleSlashCommand processes slash commands.
func (a *App) handleSlashCommand(cmd string) tea.Cmd {
	parts := strings.Fields(cmd)
	command := parts[0]

	switch command {
	case "/quit":
		a.quitting = true
		return tea.Quit
	default:
		return tea.Println(components.RenderError(fmt.Sprintf("Unknown command: %s", command), a.width))
	}
}

// isMouseEscapeFragment returns true if s looks like one or more unparsed
// SGR mouse escape sequence fragments (e.g. "[<65;80;14M" or concatenated
// "[<65;80;14M[<64;80;14M").
func isMouseEscapeFragment(s string) bool {
	if len(s) < 5 || s[0] != '[' || s[1] != '<' {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == '[', r == '<', r == ';', r == 'M', r == 'm':
		default:
			return false
		}
	}
	return true
}
