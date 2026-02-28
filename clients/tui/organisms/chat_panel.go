package organisms

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ChatPanelStyles contains the styles injected into the ChatPanel.
type ChatPanelStyles struct {
	Assistant  lipgloss.Style
	User       lipgloss.Style
	Error      lipgloss.Style
	Muted      lipgloss.Style
	ToolBorder lipgloss.Style
}

// ChatPanel manages the conversation viewport, blocks, and stream logic.
type ChatPanel struct {
	viewport    OutputViewport
	spinner     spinner.Model
	streamed    bool
	streamBlock *TextBlock // active streaming block (survives interleaved tool blocks)
	width       int
	styles      ChatPanelStyles
}

// NewChatPanel creates a new chat panel.
func NewChatPanel(width, height int, styles ChatPanelStyles) ChatPanel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return ChatPanel{
		viewport: NewOutputViewport(width, height),
		spinner:  s,
		width:    width,
		styles:   styles,
	}
}

// Init returns the first spinner tick command.
func (p ChatPanel) Init() tea.Cmd {
	return p.spinner.Tick
}

// HandleStreamStart begins a new streaming assistant block.
func (p *ChatPanel) HandleStreamStart() {
	p.streamed = true
	block := NewTextBlock("Ozzie", p.styles.Assistant, p.width)
	p.streamBlock = block
	p.viewport.AppendBlock(block)
}

// HandleStreamDelta appends content to the current streaming block.
// Uses the tracked streamBlock so interleaved tool blocks don't break streaming.
func (p *ChatPanel) HandleStreamDelta(content string) {
	if p.streamBlock != nil {
		p.streamBlock.AppendDelta(content)
		p.viewport.Refresh()
	}
}

// HandleStreamEnd marks the current streaming block as complete.
func (p *ChatPanel) HandleStreamEnd() {
	if p.streamBlock != nil {
		p.streamBlock.SetComplete()
		p.streamBlock = nil
		p.viewport.Refresh()
	}
}

// HandleStreamInterrupt marks the current stream as interrupted.
func (p *ChatPanel) HandleStreamInterrupt() {
	if p.streamBlock != nil {
		p.streamBlock.AppendDelta(" (interrupted)")
		p.streamBlock.SetComplete()
		p.streamBlock = nil
		p.viewport.Refresh()
	}
}

// CompleteLastStreamBlock marks the active streaming block as complete if it isn't already.
// Handles the case where StreamEnd is never received (e.g. LLM timeout).
func (p *ChatPanel) CompleteLastStreamBlock() {
	if p.streamBlock != nil && !p.streamBlock.IsComplete() {
		p.streamBlock.SetComplete()
		p.streamBlock = nil
		p.viewport.Refresh()
	}
}

// HandleAssistantMessage handles a complete assistant message, deduplicating with streaming.
func (p *ChatPanel) HandleAssistantMessage(content, errMsg string) {
	p.CompleteLastStreamBlock()
	if errMsg != "" {
		block := NewTextBlock("Error", p.styles.Error, p.width)
		block.AppendDelta(errMsg)
		block.SetComplete()
		p.viewport.AppendBlock(block)
		p.streamed = false
		return
	}
	if p.streamed {
		p.streamed = false
		return
	}
	if content != "" {
		block := NewTextBlock("Ozzie", p.styles.Assistant, p.width)
		block.AppendDelta(content)
		block.SetComplete()
		p.viewport.AppendBlock(block)
	}
}

// HandleToolCall manages tool call lifecycle events.
func (p *ChatPanel) HandleToolCall(status, name string, args map[string]any, result, errMsg string) {
	switch status {
	case "started":
		block := NewToolBlock(name, args, p.styles.ToolBorder)
		p.viewport.AppendBlock(block)
	case "completed", "failed":
		for i := p.viewport.BlockCount() - 1; i >= 0; i-- {
			if tb, ok := p.viewport.Block(i).(*ToolBlock); ok && tb.Name() == name {
				tb.UpdateStatus(status, result, errMsg)
				p.viewport.Refresh()
				break
			}
		}
	}
}

// HandleSkillStarted adds a skill block.
func (p *ChatPanel) HandleSkillStarted(name string) {
	block := NewSkillBlock(name, p.styles.ToolBorder)
	p.viewport.AppendBlock(block)
}

// HandleSkillStepStarted adds a step to a skill block.
func (p *ChatPanel) HandleSkillStepStarted(skillName, stepID, stepTitle string) {
	if sb := p.findSkillBlock(skillName); sb != nil {
		sb.AddStep(stepID, stepTitle)
		p.viewport.Refresh()
	}
}

// HandleSkillStepCompleted marks a skill step as done.
func (p *ChatPanel) HandleSkillStepCompleted(skillName, stepID string, duration time.Duration, errMsg string) {
	if sb := p.findSkillBlock(skillName); sb != nil {
		sb.CompleteStep(stepID, duration, errMsg)
		p.viewport.Refresh()
	}
}

// HandleSkillCompleted marks a skill as done.
func (p *ChatPanel) HandleSkillCompleted(name string, duration time.Duration, errMsg string) {
	if sb := p.findSkillBlock(name); sb != nil {
		sb.SetComplete(duration, errMsg)
		p.viewport.Refresh()
	}
}

func (p *ChatPanel) findSkillBlock(name string) *SkillBlock {
	for i := p.viewport.BlockCount() - 1; i >= 0; i-- {
		if sb, ok := p.viewport.Block(i).(*SkillBlock); ok && sb.Name() == name {
			return sb
		}
	}
	return nil
}

// AppendUserMessage adds a user message block.
func (p *ChatPanel) AppendUserMessage(content string) {
	block := NewTextBlock("You", p.styles.User, p.width)
	block.AppendDelta(content)
	block.SetComplete()
	p.viewport.AppendBlock(block)
}

// AppendErrorMessage adds an error message block.
func (p *ChatPanel) AppendErrorMessage(content string) {
	block := NewTextBlock("Error", p.styles.Error, p.width)
	block.AppendDelta(content)
	block.SetComplete()
	p.viewport.AppendBlock(block)
}

// AppendSystemMessage adds a system message block.
func (p *ChatPanel) AppendSystemMessage(content string) {
	block := NewTextBlock("System", p.styles.Muted, p.width)
	block.AppendDelta(content)
	block.SetComplete()
	p.viewport.AppendBlock(block)
}

// ToggleLastToolCollapsed toggles expand/collapse on the last completed tool block.
func (p *ChatPanel) ToggleLastToolCollapsed() {
	for i := p.viewport.BlockCount() - 1; i >= 0; i-- {
		if tb, ok := p.viewport.Block(i).(*ToolBlock); ok && tb.IsComplete() {
			tb.ToggleCollapsed()
			p.viewport.Refresh()
			break
		}
	}
}

// PageUp scrolls up by one page.
func (p *ChatPanel) PageUp() { p.viewport.PageUp() }

// PageDown scrolls down by one page.
func (p *ChatPanel) PageDown() { p.viewport.PageDown() }

// ClearBlocks resets the viewport with new dimensions.
func (p *ChatPanel) ClearBlocks(w, h int) {
	p.viewport = NewOutputViewport(w, h)
	p.width = w
}

// SetSize updates the viewport dimensions.
func (p *ChatPanel) SetSize(w, h int) {
	p.width = w
	p.viewport.SetSize(w, h)
}

// Streamed returns whether the current response was streamed.
func (p *ChatPanel) Streamed() bool { return p.streamed }

// ResetStreamed clears the streamed flag.
func (p *ChatPanel) ResetStreamed() { p.streamed = false }

// Update handles spinner ticks and viewport passthrough.
func (p ChatPanel) Update(msg tea.Msg) (ChatPanel, tea.Cmd) {
	var cmds []tea.Cmd

	if _, ok := msg.(spinner.TickMsg); ok {
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		cmds = append(cmds, cmd)
		p.viewport.Refresh()
	}

	var vpCmd tea.Cmd
	p.viewport, vpCmd = p.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return p, tea.Batch(cmds...)
}

// View renders the chat viewport.
func (p ChatPanel) View() string {
	return p.viewport.View()
}
