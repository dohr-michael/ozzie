// Package components provides reusable TUI components.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// MessageType represents the type of chat message.
type MessageType int

const (
	MessageTypeUser MessageType = iota
	MessageTypeAssistant
	MessageTypeSystem
	MessageTypeTool
	MessageTypeAnswer // Wizard/prompt answers ("> Label: Value")
)

// ToolCallStatus represents the state of a tool call.
type ToolCallStatus int

const (
	ToolStatusPending ToolCallStatus = iota
	ToolStatusRunning
	ToolStatusAwaitingConfirmation
	ToolStatusConfirmed
	ToolStatusDenied
	ToolStatusCompleted
	ToolStatusFailed
)

// ToolCall represents a tool invocation with its result.
type ToolCall struct {
	Name      string
	Arguments string
	Result    string
	Error     error
	Status    ToolCallStatus
	Completed bool // Keep for backward compat
}

// InterimMessage represents a message that appears during an interaction (e.g., field confirmations).
type InterimMessage struct {
	Content       string
	CorrelationID string
}

// MessageGroup represents a group of messages from a single interaction.
// This includes the user message, tool calls, and assistant response.
type MessageGroup struct {
	UserMessage      string
	ToolCalls        []ToolCall
	InterimMessages  []InterimMessage // Messages that appear during the interaction (after tools)
	AssistantMessage string
	Collapsed        bool   // Whether tool calls are collapsed
	Completed        bool   // Whether the interaction is complete
	IsError          bool   // Whether the response is an error
	IsAnswer         bool   // Whether this is a wizard/prompt answer
	CorrelationID    string // Optional ID for grouping related messages
}

// toolGroupRegion tracks the clickable region for a tool group.
type toolGroupRegion struct {
	groupIdx  int // Index in groups slice
	startLine int // Start line in rendered content
	endLine   int // End line (exclusive)
}

// Chat is a scrollable chat history component with collapsible tool groups.
type Chat struct {
	viewport viewport.Model
	groups   []MessageGroup
	current  *MessageGroup // Current in-progress interaction

	// Streaming state
	streaming string

	// UI state
	width        int
	height       int
	ready        bool
	autoScroll   bool
	showThinking bool
	showWelcome  bool

	// Clickable regions for tool groups
	toolRegions []toolGroupRegion
}

// NewChat creates a new chat component.
func NewChat() *Chat {
	return &Chat{
		groups:      make([]MessageGroup, 0),
		autoScroll:  true,
		showWelcome: true,
	}
}

// Init initializes the chat component.
func (c *Chat) Init() tea.Cmd {
	return nil
}

// Update handles messages.
// Note: Size is managed by the parent App via SetSize(), not WindowSizeMsg.
func (c *Chat) Update(msg tea.Msg) (*Chat, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "pgup", "pgdown", "up", "down":
			c.viewport, cmd = c.viewport.Update(msg)
			cmds = append(cmds, cmd)
			c.autoScroll = c.viewport.AtBottom()
		case "tab":
			// Toggle collapse on last completed group
			if len(c.groups) > 0 {
				lastIdx := len(c.groups) - 1
				if c.groups[lastIdx].Completed && len(c.groups[lastIdx].ToolCalls) > 0 {
					c.groups[lastIdx].Collapsed = !c.groups[lastIdx].Collapsed
					c.refreshContent()
				}
			}
		}

	case tea.MouseMsg:
		// Check for left click on tool groups
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
			// Calculate actual line in content (viewport offset + click Y)
			contentLine := c.viewport.YOffset + msg.Y

			// Check if click is on a tool group region
			for _, region := range c.toolRegions {
				if contentLine >= region.startLine && contentLine < region.endLine {
					// Toggle this group's collapsed state
					if region.groupIdx < len(c.groups) {
						c.groups[region.groupIdx].Collapsed = !c.groups[region.groupIdx].Collapsed
						c.refreshContent()
					}
					break
				}
			}
		}

		c.viewport, cmd = c.viewport.Update(msg)
		cmds = append(cmds, cmd)
		c.autoScroll = c.viewport.AtBottom()

	// Component messages - preferred way to update state
	case ChatStartInteractionMsg:
		c.StartInteraction(msg.UserMessage)

	case ChatAddToolCallMsg:
		c.AddToolCall(msg.Name, msg.Arguments)

	case ChatAddToolResultMsg:
		c.AddToolResult(msg.Name, msg.Result, msg.Error)

	case ChatSetToolAwaitingMsg:
		c.SetToolAwaitingConfirmation(msg.Name, msg.Arguments)

	case ChatSetStreamingMsg:
		c.SetStreaming(msg.Content)

	case ChatCompleteInteractionMsg:
		if msg.IsError {
			c.CompleteInteractionWithError(msg.Message)
		} else {
			c.CompleteInteraction(msg.Message)
		}

	case ChatSetThinkingMsg:
		c.SetThinking(msg.Thinking)

	case ChatClearMsg:
		c.Clear()

	case ChatAddCorrelatedMsg:
		c.AddMessageWithCorrelation(msg.Content, msg.CorrelationID, msg.ClearsCorrelation)
	}

	return c, tea.Batch(cmds...)
}

// View renders the chat component.
func (c *Chat) View() string {
	if !c.ready {
		return "Initializing..."
	}
	return c.viewport.View()
}

// SetSize updates the component size.
func (c *Chat) SetSize(width, height int) {
	c.width = width
	c.height = height

	if !c.ready {
		c.viewport = viewport.New(width, height)
		c.viewport.SetContent(c.renderContent())
		c.ready = true
	} else {
		c.viewport.Width = width
		c.viewport.Height = height
		c.viewport.SetContent(c.renderContent())
	}
}

// StartInteraction starts a new user interaction.
func (c *Chat) StartInteraction(userMessage string) {
	// If there's an existing interaction in progress, save it first
	if c.current != nil && c.current.UserMessage != "" {
		c.current.Completed = true
		c.groups = append(c.groups, *c.current)
	}

	c.current = &MessageGroup{
		UserMessage: userMessage,
		ToolCalls:   make([]ToolCall, 0),
		Collapsed:   false,
		Completed:   false,
	}
	c.refreshContent()
}

// AddToolCall adds a tool call to the current interaction.
func (c *Chat) AddToolCall(name, args string) {
	if c.current == nil {
		c.current = &MessageGroup{
			ToolCalls: make([]ToolCall, 0),
		}
	}
	c.current.ToolCalls = append(c.current.ToolCalls, ToolCall{
		Name:      name,
		Arguments: args,
		Completed: false,
	})
	c.refreshContent()
}

// AddToolResult updates the last tool call with its result.
func (c *Chat) AddToolResult(name, result string, err error) {
	if c.current == nil {
		return
	}

	// Find the matching tool call and update it
	for i := len(c.current.ToolCalls) - 1; i >= 0; i-- {
		if c.current.ToolCalls[i].Name == name && !c.current.ToolCalls[i].Completed {
			c.current.ToolCalls[i].Result = result
			c.current.ToolCalls[i].Error = err
			c.current.ToolCalls[i].Completed = true
			if err != nil {
				c.current.ToolCalls[i].Status = ToolStatusFailed
			} else {
				c.current.ToolCalls[i].Status = ToolStatusCompleted
			}
			break
		}
	}
	c.refreshContent()
}

// SetToolAwaitingConfirmation marks a tool as awaiting user confirmation.
func (c *Chat) SetToolAwaitingConfirmation(name, args string) {
	if c.current == nil {
		c.current = &MessageGroup{
			ToolCalls: make([]ToolCall, 0),
		}
	}
	c.current.ToolCalls = append(c.current.ToolCalls, ToolCall{
		Name:      name,
		Arguments: args,
		Status:    ToolStatusAwaitingConfirmation,
	})
	c.refreshContent()
}

// SetToolConfirmed marks the last awaiting tool as confirmed or denied.
func (c *Chat) SetToolConfirmed(name string, allowed bool) {
	if c.current == nil {
		return
	}

	// Find the matching tool awaiting confirmation
	for i := len(c.current.ToolCalls) - 1; i >= 0; i-- {
		tc := &c.current.ToolCalls[i]
		if tc.Name == name && tc.Status == ToolStatusAwaitingConfirmation {
			if allowed {
				tc.Status = ToolStatusConfirmed
			} else {
				tc.Status = ToolStatusDenied
				tc.Completed = true
			}
			break
		}
	}
	c.refreshContent()
}

// SetStreaming sets the current streaming content.
func (c *Chat) SetStreaming(content string) {
	c.streaming = content
	c.refreshContent()
}

// CompleteInteraction finishes the current interaction and collapses tools.
func (c *Chat) CompleteInteraction(assistantMessage string) {
	c.completeInteraction(assistantMessage, false)
}

// CompleteInteractionWithError finishes the interaction with an error message.
func (c *Chat) CompleteInteractionWithError(errorMessage string) {
	c.completeInteraction(errorMessage, true)
}

// completeInteraction is the internal method to finish an interaction.
func (c *Chat) completeInteraction(message string, isError bool) {
	if c.current == nil {
		c.current = &MessageGroup{}
	}

	c.current.AssistantMessage = message
	c.current.Completed = true
	c.current.IsError = isError
	c.current.Collapsed = len(c.current.ToolCalls) > 0 // Auto-collapse if there were tools

	c.groups = append(c.groups, *c.current)
	c.current = nil
	c.streaming = ""
	c.showThinking = false
	c.refreshContent()
}

// SetThinking shows/hides the thinking indicator.
func (c *Chat) SetThinking(thinking bool) {
	c.showThinking = thinking
	c.refreshContent()
}

// Clear removes all messages.
func (c *Chat) Clear() {
	c.groups = nil
	c.current = nil
	c.streaming = ""
	c.showThinking = false
	c.showWelcome = true
	c.refreshContent()
}

// ClearStreaming clears the streaming content (compatibility method).
func (c *Chat) ClearStreaming(addAsMessage bool) {
	if addAsMessage && c.streaming != "" {
		c.CompleteInteraction(c.streaming)
	} else {
		c.streaming = ""
		c.refreshContent()
	}
}

// AddMessage adds a simple message (compatibility method).
func (c *Chat) AddMessage(role, content string) {
	switch role {
	case "user":
		c.StartInteraction(content)
	case "assistant":
		c.CompleteInteraction(content)
	case "system":
		// System messages go directly to groups as a single-message group
		c.groups = append(c.groups, MessageGroup{
			AssistantMessage: content,
			Completed:        true,
		})
		c.refreshContent()
	}
}

// AddMessageWithCorrelation adds a message with an optional correlation ID.
// If there's a current interaction in progress, adds as an interim message.
// If clearsCorrelation is true, all previous messages/interim messages with that ID are removed first.
func (c *Chat) AddMessageWithCorrelation(content, correlationID string, clearsCorrelation bool) {
	if clearsCorrelation && correlationID != "" {
		c.clearByCorrelationID(correlationID)
	}

	// If there's a current interaction, add as interim message
	if c.current != nil {
		c.current.InterimMessages = append(c.current.InterimMessages, InterimMessage{
			Content:       content,
			CorrelationID: correlationID,
		})
	} else {
		// No current interaction, add as standalone group
		c.groups = append(c.groups, MessageGroup{
			AssistantMessage: content,
			Completed:        true,
			CorrelationID:    correlationID,
		})
	}
	c.refreshContent()
}

// clearByCorrelationID removes all message groups and interim messages with the given correlation ID.
func (c *Chat) clearByCorrelationID(correlationID string) {
	if correlationID == "" {
		return
	}

	// Clear from completed groups
	filtered := make([]MessageGroup, 0, len(c.groups))
	for _, g := range c.groups {
		if g.CorrelationID != correlationID {
			// Also filter interim messages within the group
			filteredInterim := make([]InterimMessage, 0, len(g.InterimMessages))
			for _, im := range g.InterimMessages {
				if im.CorrelationID != correlationID {
					filteredInterim = append(filteredInterim, im)
				}
			}
			g.InterimMessages = filteredInterim
			filtered = append(filtered, g)
		}
	}
	c.groups = filtered

	// Clear from current interaction if exists
	if c.current != nil {
		filteredInterim := make([]InterimMessage, 0, len(c.current.InterimMessages))
		for _, im := range c.current.InterimMessages {
			if im.CorrelationID != correlationID {
				filteredInterim = append(filteredInterim, im)
			}
		}
		c.current.InterimMessages = filteredInterim
	}
}

// Messages returns legacy message format (compatibility).
func (c *Chat) Messages() []Message {
	var msgs []Message
	for _, g := range c.groups {
		if g.UserMessage != "" {
			msgs = append(msgs, Message{Role: "user", Content: g.UserMessage})
		}
		if g.AssistantMessage != "" {
			msgs = append(msgs, Message{Role: "assistant", Content: g.AssistantMessage})
		}
	}
	return msgs
}

// refreshContent updates the viewport content.
func (c *Chat) refreshContent() {
	if !c.ready {
		return
	}
	c.viewport.SetContent(c.renderContent())
	if c.autoScroll {
		c.viewport.GotoBottom()
	}
}

// renderContent renders the full chat content and tracks clickable regions.
func (c *Chat) renderContent() string {
	var b strings.Builder
	currentLine := 0

	// Reset tool regions
	c.toolRegions = nil

	// Helper to count lines in a string
	countLines := func(s string) int {
		if s == "" {
			return 0
		}
		return strings.Count(s, "\n") + 1
	}

	// Welcome message
	if c.showWelcome {
		welcome := c.renderWelcome()
		b.WriteString(welcome)
		b.WriteString("\n\n")
		currentLine += countLines(welcome) + 2
	}

	// Render completed groups
	for i, group := range c.groups {
		rendered := c.renderGroupWithRegion(group, i, currentLine)
		b.WriteString(rendered)
		b.WriteString("\n")
		currentLine += countLines(rendered) + 1
	}

	// Render current in-progress interaction
	if c.current != nil {
		current := c.renderCurrentInteraction()
		b.WriteString(current)
		currentLine += countLines(current)
	}

	// Show thinking indicator
	if c.showThinking && c.streaming == "" && (c.current == nil || len(c.current.ToolCalls) == 0) {
		b.WriteString(c.renderThinking())
	}

	return b.String()
}

// renderGroupWithRegion renders a group and tracks the tool region for clicking.
func (c *Chat) renderGroupWithRegion(g MessageGroup, groupIdx, startLine int) string {
	var b strings.Builder
	currentLine := startLine

	// User message
	if g.UserMessage != "" {
		userMsg := c.renderUserMessage(g.UserMessage)
		b.WriteString(userMsg)
		b.WriteString("\n\n")
		currentLine += strings.Count(userMsg, "\n") + 3
	}

	// Tool calls (collapsed or expanded) - track region
	if len(g.ToolCalls) > 0 {
		toolStartLine := currentLine

		var toolContent string
		if g.Collapsed {
			toolContent = c.renderCollapsedTools(g.ToolCalls)
		} else {
			toolContent = c.renderExpandedTools(g.ToolCalls)
		}
		b.WriteString(toolContent)
		b.WriteString("\n")

		toolEndLine := currentLine + strings.Count(toolContent, "\n") + 2

		// Register clickable region
		c.toolRegions = append(c.toolRegions, toolGroupRegion{
			groupIdx:  groupIdx,
			startLine: toolStartLine,
			endLine:   toolEndLine,
		})

		currentLine = toolEndLine
	}

	// Interim messages (field confirmations, step completions, etc.)
	if len(g.InterimMessages) > 0 {
		b.WriteString(c.renderInterimMessages(g.InterimMessages))
		b.WriteString("\n")
	}

	// Assistant response or Answer
	if g.AssistantMessage != "" {
		if g.IsAnswer {
			b.WriteString(c.renderAnswerMessage(g.AssistantMessage))
		} else {
			b.WriteString(c.renderAssistantMessage(g.AssistantMessage, g.IsError))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderWelcome renders the welcome message (clean, no box).
func (c *Chat) renderWelcome() string {
	var b strings.Builder
	b.WriteString(WelcomeTitleStyle.Render("Ozzie"))
	b.WriteString(WelcomeSubtitleStyle.Render(" — Your personal AI agent operating system."))
	b.WriteString("\n\n")
	b.WriteString(HelpTextStyle.Render("  Tips: Ctrl+C to quit • Ctrl+L to clear"))

	return b.String()
}

// renderCurrentInteraction renders the in-progress interaction.
func (c *Chat) renderCurrentInteraction() string {
	var b strings.Builder

	// User message
	if c.current.UserMessage != "" {
		b.WriteString(c.renderUserMessage(c.current.UserMessage))
		b.WriteString("\n\n")
	}

	// Streaming content (no prefix — markdown flows naturally)
	if c.streaming != "" {
		b.WriteString(c.streaming)
		b.WriteString(SpinnerStyle.Render("▌"))
		b.WriteString("\n")
	}

	// Interim messages (field confirmations, step completions, etc.)
	if len(c.current.InterimMessages) > 0 {
		b.WriteString(c.renderInterimMessages(c.current.InterimMessages))
		b.WriteString("\n")
	}

	return b.String()
}

// renderUserMessage renders a user message with ❯ prefix.
func (c *Chat) renderUserMessage(content string) string {
	prefix := InputPromptCharStyle.Render("❯ ")
	wrapped := c.wrapText(content, c.width-4)
	lines := strings.Split(wrapped, "\n")

	var result strings.Builder
	for i, line := range lines {
		if i == 0 {
			result.WriteString(prefix + UserStyle.Render(line))
		} else {
			result.WriteString("\n  " + UserStyle.Render(line))
		}
	}
	return result.String()
}

// renderAssistantMessage renders an assistant message with markdown formatting.
func (c *Chat) renderAssistantMessage(content string, isError bool) string {
	if isError {
		wrapped := c.wrapText(content, c.width-2)
		return ErrorStyle.Render(wrapped)
	}

	// Render markdown for completed assistant messages
	rendered := RenderMarkdownWithWidth(content, c.width-2)
	return rendered
}

// renderAnswerMessage renders a wizard/prompt answer.
func (c *Chat) renderAnswerMessage(content string) string {
	return AnswerLabelStyle.Render("> ") + AnswerStyle.Render(content)
}

// renderInterimMessages renders interim messages (field confirmations, step completions).
func (c *Chat) renderInterimMessages(messages []InterimMessage) string {
	if len(messages) == 0 {
		return ""
	}

	var b strings.Builder
	for i, msg := range messages {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(InterimStyle.Render(msg.Content))
	}
	return b.String()
}

// renderCollapsedTools renders collapsed tool summary.
func (c *Chat) renderCollapsedTools(tools []ToolCall) string {
	failCount := 0
	for _, t := range tools {
		switch t.Status {
		case ToolStatusFailed:
			failCount++
		default:
			if t.Error != nil {
				failCount++
			}
		}
	}

	summary := fmt.Sprintf("▸ %d tool calls", len(tools))
	if failCount > 0 {
		summary += fmt.Sprintf(" (%d failed)", failCount)
	}

	return ToolCollapsedStyle.Render(summary)
}

// renderExpandedTools renders expanded tool calls in Claude Code style.
// Format:
//
//	⏺ ToolName(args)
//	  ⎿  result line 1
//	  ⎿  result line 2
func (c *Chat) renderExpandedTools(tools []ToolCall) string {
	var b strings.Builder

	for i, tool := range tools {
		if i > 0 {
			b.WriteString("\n")
		}

		// Bullet color depends on status
		var bullet string
		switch tool.Status {
		case ToolStatusAwaitingConfirmation:
			bullet = ConfirmWaitStyle.Render("⏺ ")
		case ToolStatusConfirmed:
			bullet = ConfirmApprovedStyle.Render("⏺ ")
		case ToolStatusDenied:
			bullet = ConfirmDeniedStyle.Render("⏺ ")
		case ToolStatusFailed:
			bullet = ToolErrorStyle.Render("⏺ ")
		case ToolStatusCompleted:
			bullet = ToolBulletStyle.Render("⏺ ")
		default:
			if tool.Completed && tool.Error != nil {
				bullet = ToolErrorStyle.Render("⏺ ")
			} else if !tool.Completed {
				bullet = ToolBulletStyle.Render("⏺ ")
			} else {
				bullet = ToolBulletStyle.Render("⏺ ")
			}
		}

		// Name(args) or Name ...
		name := ToolNameStyle.Render(tool.Name)
		if tool.Arguments != "" {
			args := TruncateString(tool.Arguments, 60)
			b.WriteString(bullet + name + ToolArgsStyle.Render("("+args+")"))
		} else {
			b.WriteString(bullet + name)
		}

		// Running indicator
		if !tool.Completed && tool.Status != ToolStatusAwaitingConfirmation &&
			tool.Status != ToolStatusConfirmed && tool.Status != ToolStatusDenied {
			b.WriteString(ToolArgsStyle.Render(" ..."))
		}

		// Status suffix for special states
		switch tool.Status {
		case ToolStatusAwaitingConfirmation:
			b.WriteString(ConfirmWaitStyle.Render(" (awaiting confirmation)"))
		case ToolStatusDenied:
			b.WriteString(ConfirmDeniedStyle.Render(" (denied)"))
		}

		// Result lines with ⎿ prefix
		if tool.Completed && tool.Error == nil {
			resultPrefix := ToolResultPrefixStyle.Render("  ⎿  ")
			if tool.Result == "" {
				b.WriteString("\n" + resultPrefix + ToolResultStyle.Render("(No output)"))
			} else {
				result := c.wrapText(tool.Result, c.width-6)
				lines := strings.Split(result, "\n")
				maxLines := 10
				for j, line := range lines {
					if j >= maxLines {
						b.WriteString("\n" + resultPrefix + ToolResultStyle.Render(fmt.Sprintf("... (%d more lines)", len(lines)-maxLines)))
						break
					}
					b.WriteString("\n" + resultPrefix + ToolResultStyle.Render(line))
				}
			}
		}

		// Error with ⎿ prefix
		if tool.Error != nil {
			resultPrefix := ToolResultPrefixStyle.Render("  ⎿  ")
			b.WriteString("\n" + resultPrefix + ToolErrorStyle.Render(tool.Error.Error()))
		}
	}

	return b.String()
}

// renderThinking renders the thinking indicator.
func (c *Chat) renderThinking() string {
	return ToolBulletStyle.Render("⏺ ") + ThinkingStyle.Render("Thinking...")
}

// wrapText wraps text to the specified width.
func (c *Chat) wrapText(text string, width int) string {
	if width <= 0 {
		width = 80
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		if line == "" {
			continue
		}

		for len(line) > width {
			breakPoint := width
			for j := width; j > 0; j-- {
				if line[j] == ' ' {
					breakPoint = j
					break
				}
			}
			result.WriteString(line[:breakPoint])
			result.WriteString("\n")
			line = strings.TrimLeft(line[breakPoint:], " ")
		}
		result.WriteString(line)
	}

	return result.String()
}

// Message is kept for backward compatibility.
type Message struct {
	Role    string
	Content string
}
