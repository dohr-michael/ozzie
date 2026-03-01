package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dohr-michael/ozzie/clients/tui/components"
	wsclient "github.com/dohr-michael/ozzie/clients/ws"
	"github.com/dohr-michael/ozzie/internal/events"
)

// App is the main TUI application model.
// Architecture: CHAT | INPUT_ZONE | FOOTER
type App struct {
	// Components
	header    *components.Header
	chat      *components.Chat
	inputZone *components.InputZone

	// State
	width            int
	height           int
	streaming        bool
	streamingContent string
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
		chat:      components.NewChat(),
		inputZone: components.NewInputZone(),
		client:    client,
		sessionID: sessionID,
	}
}

// Init initializes the application.
func (a *App) Init() tea.Cmd {
	return tea.Batch(a.inputZone.Init(), a.inputZone.Focus())
}

// Update handles messages and updates state.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updateSizes()
		if a.inputZone.Mode() == components.ModeChat && !a.streaming {
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
		case "ctrl+l":
			a.chat.Clear()
			return a, nil
		case "pgup", "pgdown":
			var chatCmd tea.Cmd
			a.chat, chatCmd = a.chat.Update(msg)
			cmds = append(cmds, chatCmd)
			return a, tea.Batch(cmds...)
		}

		// Update input zone
		var cmd tea.Cmd
		a.inputZone, cmd = a.inputZone.Update(msg)
		cmds = append(cmds, cmd)

		// Guard: don't pass up/down to chat when in navigation mode
		mode := a.inputZone.Mode()
		isNavigationMode := mode == components.ModeSelect || mode == components.ModeMulti || mode == components.ModeConfirm
		isNavigationKey := msg.String() == "up" || msg.String() == "down"
		if isNavigationMode && isNavigationKey {
			return a, tea.Batch(cmds...)
		}

	case tea.MouseMsg:
		var chatCmd tea.Cmd
		a.chat, chatCmd = a.chat.Update(msg)
		cmds = append(cmds, chatCmd)

	case components.InputResult:
		cmds = append(cmds, a.handleInputResult(msg))

	// --- Ozzie WS messages ---

	case StreamStartMsg:
		a.streaming = true
		a.streamingContent = ""
		a.header.SetStreaming(true)
		a.chat.SetThinking(true)
		a.inputZone.SetDisabled(true)

	case StreamDeltaMsg:
		a.streamingContent += msg.Content
		a.chat.SetStreaming(a.streamingContent)

	case StreamEndMsg:
		// No-op: content finalized by AssistantMessageMsg

	case AssistantMessageMsg:
		a.streaming = false
		a.header.SetStreaming(false)
		a.inputZone.SetDisabled(false)
		a.chat.SetThinking(false)

		if msg.Error != "" {
			a.chat.CompleteInteractionWithError(msg.Error)
		} else {
			a.chat.CompleteInteraction(msg.Content)
		}
		a.streamingContent = ""
		cmds = append(cmds, a.inputZone.Focus())
		return a, tea.Batch(cmds...)

	case ToolCallMsg:
		a.handleToolCall(msg)

	case PromptRequestMsg:
		a.handlePromptRequest(msg)

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
		a.chat.AddMessage("system", fmt.Sprintf("Send error: %v", msg.err))
		return a, nil
	}

	// Fallthrough: always update chat to handle viewport/framework messages
	var chatCmd tea.Cmd
	a.chat, chatCmd = a.chat.Update(msg)
	cmds = append(cmds, chatCmd)

	return a, tea.Batch(cmds...)
}

// View renders the application: CHAT | INPUT | FOOTER.
func (a *App) View() string {
	if a.quitting {
		return "Goodbye!\n"
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		a.chat.View(),
		a.inputZone.View(),
		a.header.View(),
	)
}

func (a *App) updateSizes() {
	footerHeight := 1

	// Dynamic input height based on current mode
	inputHeight := a.inputHeightForMode(a.inputZone.Mode())

	chatHeight := a.height - footerHeight - inputHeight
	if chatHeight < 5 {
		chatHeight = 5
	}

	a.header.SetWidth(a.width)
	a.chat.SetSize(a.width, chatHeight)
	a.inputZone.SetSize(a.width, inputHeight)
}

// inputHeightForMode returns the appropriate input zone height for a given mode.
func (a *App) inputHeightForMode(mode components.InputMode) int {
	switch mode {
	case components.ModeChat:
		return 3 // sep + input + sep
	case components.ModeConfirm:
		return 7 // sep + question + yes + no + hint + sep + margin
	case components.ModeText:
		return 7 // sep + question + input + error + hint + sep + margin
	case components.ModeSelect, components.ModeMulti:
		return 10 // sep + question + ~5 options + hint + sep + margin
	default:
		return 3
	}
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

		a.chat.StartInteraction(text)
		a.inputZone.SetDisabled(true)
		a.chat.SetThinking(true)
		a.streaming = true
		a.streamingContent = ""
		a.header.SetStreaming(true)

		client := a.client
		return func() tea.Msg {
			if err := client.SendMessage(text); err != nil {
				return sendErrorMsg{err: err}
			}
			return nil
		}

	case components.ModeConfirm:
		token := a.currentPromptToken
		if result.ResumeToken != "" {
			token = result.ResumeToken
		}
		a.currentPromptToken = ""
		a.updateSizes()

		if result.Confirmed {
			a.chat.SetThinking(true)
			a.streaming = true
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
		a.chat.SetThinking(true)
		a.streaming = true
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
		a.chat.SetThinking(true)
		a.streaming = true
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
		a.chat.SetThinking(true)
		a.streaming = true
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

// handleToolCall dispatches tool call events to the chat component.
func (a *App) handleToolCall(msg ToolCallMsg) {
	switch msg.Status {
	case string(events.ToolStatusStarted):
		args := components.FormatArguments(msg.Arguments)
		a.chat.AddToolCall(msg.Name, args)
	case string(events.ToolStatusCompleted):
		a.chat.AddToolResult(msg.Name, msg.Result, nil)
	case string(events.ToolStatusFailed):
		a.chat.AddToolResult(msg.Name, "", fmt.Errorf("%s", msg.Error))
	}
}

// handlePromptRequest bridges PromptRequestMsg to InputZone prompts.
func (a *App) handlePromptRequest(msg PromptRequestMsg) {
	a.streaming = false
	a.header.SetStreaming(false)
	a.chat.SetThinking(false)
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
	case "/clear":
		a.chat.Clear()
	default:
		a.chat.AddMessage("system", fmt.Sprintf("Unknown command: %s", command))
	}

	return nil
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
