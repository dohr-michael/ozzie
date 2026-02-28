package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	wsclient "github.com/dohr-michael/ozzie/clients/ws"

	"github.com/dohr-michael/ozzie/clients/tui/molecules"
	"github.com/dohr-michael/ozzie/clients/tui/organisms"
)

// MainModel is the root bubbletea model for the Ozzie TUI.
type MainModel struct {
	client *wsclient.Client
	mode   organisms.Mode
	width  int
	height int

	chat        organisms.ChatPanel
	interaction organisms.InteractionPanel
	info        organisms.InformationPanel
}

// NewMainModel creates the root model.
func NewMainModel(client *wsclient.Client, sessionID string) MainModel {
	styles := organisms.ChatPanelStyles{
		Assistant:  AssistantStyle,
		User:       UserStyle,
		Error:      ErrorStyle,
		Muted:      MutedStyle,
		ToolBorder: ToolBorderStyle,
	}

	info := organisms.NewInformationPanel(StatusBarStyle)
	info.SetSession(sessionID)

	return MainModel{
		client:      client,
		mode:        organisms.ModeNormal,
		chat:        organisms.NewChatPanel(80, 20, styles),
		interaction: organisms.NewInteractionPanel(PromptBorderStyle),
		info:        info,
	}
}

// Init starts the spinner ticks.
func (m MainModel) Init() tea.Cmd {
	return m.chat.Init()
}

// Update processes all incoming messages.
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		viewportHeight := m.height - 2 // input(1) + statusbar(1)
		if viewportHeight < 1 {
			viewportHeight = 1
		}
		m.chat.SetSize(m.width, viewportHeight)
		m.interaction.SetWidth(m.width)
		m.info.SetWidth(m.width)
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m.handleKey(msg)

	case StreamStartMsg:
		m.mode = organisms.ModeStreaming
		m.chat.HandleStreamStart()
		m.info.SetMode(organisms.ModeStreaming)
		return m, nil

	case StreamDeltaMsg:
		m.chat.HandleStreamDelta(msg.Content)
		return m, nil

	case StreamEndMsg:
		m.mode = organisms.ModeNormal
		m.chat.HandleStreamEnd()
		m.info.SetMode(organisms.ModeNormal)
		return m.flushPending()

	case AssistantMessageMsg:
		m.mode = organisms.ModeNormal
		m.chat.HandleAssistantMessage(msg.Content, msg.Error)
		m.info.SetMode(organisms.ModeNormal)
		return m.flushPending()

	case ToolCallMsg:
		m.chat.HandleToolCall(msg.Status, msg.Name, msg.Arguments, msg.Result, msg.Error)
		return m, nil

	case SkillStartedMsg:
		m.chat.HandleSkillStarted(msg.Name)
		return m, nil

	case SkillStepStartedMsg:
		m.chat.HandleSkillStepStarted(msg.SkillName, msg.StepID, msg.StepTitle)
		return m, nil

	case SkillStepCompletedMsg:
		m.chat.HandleSkillStepCompleted(msg.SkillName, msg.StepID, msg.Duration, msg.Error)
		return m, nil

	case SkillCompletedMsg:
		m.chat.HandleSkillCompleted(msg.Name, msg.Duration, msg.Error)
		return m, nil

	case PromptRequestMsg:
		m.mode = organisms.ModePrompting
		m.interaction.ActivateFormWithOpts(organisms.FormActivateOpts{
			PromptType:  msg.Type,
			Label:       msg.Label,
			Token:       msg.Token,
			Options:     msg.Options,
			HelpText:    msg.HelpText,
			Placeholder: msg.Placeholder,
			MinSelect:   msg.MinSelect,
			MaxSelect:   msg.MaxSelect,
		})
		m.info.SetMode(organisms.ModePrompting)
		return m, nil

	case organisms.FormResponseMsg:
		m.mode = organisms.ModeNormal
		m.interaction.DeactivateForm()
		m.info.SetMode(organisms.ModeNormal)
		return m, m.sendPromptResponse(msg)

	case molecules.SubmitMsg:
		return m.handleSubmit(msg)

	case LLMTelemetryMsg:
		m.info.AddTokens(msg.TokensIn, msg.TokensOut)
		if msg.Model != "" {
			m.info.SetModel(msg.Model)
		}
		return m, nil

	case ConnectedMsg:
		if msg.Client != nil {
			m.client = msg.Client
		}
		m.info.SetSession(msg.SessionID)
		m.info.SetConnected(true, nil)
		return m, nil

	case DisconnectedMsg:
		m.info.SetConnected(false, msg.Err)
		return m, nil
	}

	// Pass through to chat panel (spinner ticks, viewport).
	var cmd tea.Cmd
	m.chat, cmd = m.chat.Update(msg)

	// Also pass through to active interaction sub-component.
	if m.interaction.FormActive() {
		var formCmd tea.Cmd
		m.interaction, formCmd = m.interaction.UpdateForm(msg)
		return m, tea.Batch(cmd, formCmd)
	}

	return m, cmd
}

func (m MainModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyCtrlO:
		m.chat.ToggleLastToolCollapsed()
		return m, nil

	case tea.KeyPgUp:
		m.chat.PageUp()
		return m, nil

	case tea.KeyPgDown:
		m.chat.PageDown()
		return m, nil

	case tea.KeyEsc:
		if m.interaction.FormActive() {
			var cmd tea.Cmd
			m.interaction, cmd = m.interaction.UpdateForm(msg)
			return m, cmd
		}
		if m.mode == organisms.ModeStreaming {
			m.mode = organisms.ModeNormal
			m.chat.HandleStreamInterrupt()
			m.info.SetMode(organisms.ModeNormal)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.interaction, cmd = m.interaction.Update(msg)
	return m, cmd
}

func (m MainModel) handleSubmit(msg molecules.SubmitMsg) (tea.Model, tea.Cmd) {
	if strings.HasPrefix(msg.Content, "/") {
		return m.handleSlashCommand(msg.Content)
	}

	m.chat.AppendUserMessage(msg.Content)

	if m.mode == organisms.ModeStreaming {
		m.interaction.BufferMessage(msg.Content)
		return m, nil
	}

	if err := m.client.SendMessage(msg.Content); err != nil {
		m.chat.AppendErrorMessage(fmt.Sprintf("send message: %v", err))
	}

	return m, nil
}

// flushPending sends any buffered messages that were typed during streaming.
func (m MainModel) flushPending() (tea.Model, tea.Cmd) {
	pending := m.interaction.DrainPending()
	for _, content := range pending {
		if err := m.client.SendMessage(content); err != nil {
			m.chat.AppendErrorMessage(fmt.Sprintf("send message: %v", err))
		}
	}
	return m, nil
}

func (m MainModel) handleSlashCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	command := parts[0]

	switch command {
	case "/quit":
		return m, tea.Quit

	case "/clear":
		m.chat.ClearBlocks(m.width, m.height-2)
		return m, nil

	case "/session":
		m.chat.AppendSystemMessage(fmt.Sprintf("Session: %s", m.info.SessionID()))
		return m, nil

	case "/status":
		info := fmt.Sprintf("Session: %s\nModel: %s\nTokens: %d in / %d out",
			m.info.SessionID(), m.info.Model(), m.info.TokensIn(), m.info.TokensOut())
		if !m.info.Connected() {
			info += fmt.Sprintf("\nConnection: disconnected (%v)", m.info.ConnErr())
		} else {
			info += "\nConnection: connected"
		}
		m.chat.AppendSystemMessage(info)
		return m, nil

	default:
		m.chat.AppendSystemMessage(fmt.Sprintf("Unknown command: %s", command))
		return m, nil
	}
}

func (m MainModel) sendPromptResponse(msg organisms.FormResponseMsg) tea.Cmd {
	return func() tea.Msg {
		if msg.Cancelled {
			_ = m.client.RespondToPrompt(msg.Token, true)
		} else if msg.Value == "true" {
			_ = m.client.RespondToPrompt(msg.Token, false)
		} else {
			// Check if value is a JSON array (multi-select response)
			if strings.HasPrefix(msg.Value, "[") {
				var values []string
				if err := json.Unmarshal([]byte(msg.Value), &values); err == nil {
					_ = m.client.RespondToPromptWithValues(msg.Token, values)
					return nil
				}
			}
			_ = m.client.RespondToPromptWithValue(msg.Token, msg.Value)
		}
		return nil
	}
}

// View renders the full TUI layout.
func (m MainModel) View() string {
	return fmt.Sprintf("%s\n%s\n%s", m.chat.View(), m.interaction.View(), m.info.View())
}
