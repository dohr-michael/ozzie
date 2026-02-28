// Package molecules provides mid-level TUI components.
package molecules

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SubmitMsg is sent when the user presses Enter to submit input.
type SubmitMsg struct {
	Content string
}

// CommandInput wraps a textarea with Enter-to-submit semantics.
type CommandInput struct {
	textarea textarea.Model
	enabled  bool
	history  []string
	histIdx  int
	draft    string
}

// NewCommandInput creates a new input area.
func NewCommandInput() CommandInput {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Prompt = "> "
	ta.ShowLineNumbers = false
	ta.SetHeight(1)
	ta.CharLimit = 0
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.Focus()

	return CommandInput{
		textarea: ta,
		enabled:  true,
		histIdx:  -1,
	}
}

// SetWidth sets the input width.
func (c *CommandInput) SetWidth(w int) {
	c.textarea.SetWidth(w)
}

// SetEnabled enables or disables the input.
func (c *CommandInput) SetEnabled(enabled bool) {
	c.enabled = enabled
	if enabled {
		c.textarea.Focus()
	} else {
		c.textarea.Blur()
	}
}

// Enabled returns whether the input is active.
func (c *CommandInput) Enabled() bool {
	return c.enabled
}

// Focus gives focus to the input.
func (c *CommandInput) Focus() {
	c.textarea.Focus()
}

// Blur removes focus from the input.
func (c *CommandInput) Blur() {
	c.textarea.Blur()
}

// Reset clears the input.
func (c *CommandInput) Reset() {
	c.textarea.Reset()
	c.histIdx = -1
	c.draft = ""
}

// Value returns the current input text.
func (c *CommandInput) Value() string {
	return c.textarea.Value()
}

// Update handles key events. Enter submits, Alt+Enter inserts a newline.
func (c CommandInput) Update(msg tea.Msg) (CommandInput, tea.Cmd) {
	if !c.enabled {
		return c, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEnter:
			content := strings.TrimSpace(c.textarea.Value())
			if content == "" {
				return c, nil
			}
			c.history = append(c.history, content)
			c.histIdx = -1
			c.draft = ""
			c.textarea.Reset()
			return c, func() tea.Msg { return SubmitMsg{Content: content} }

		case tea.KeyUp:
			if len(c.history) == 0 {
				break
			}
			if c.histIdx == -1 {
				c.draft = c.textarea.Value()
				c.histIdx = len(c.history) - 1
			} else if c.histIdx > 0 {
				c.histIdx--
			}
			c.textarea.SetValue(c.history[c.histIdx])
			return c, nil

		case tea.KeyDown:
			if c.histIdx == -1 {
				break
			}
			if c.histIdx < len(c.history)-1 {
				c.histIdx++
				c.textarea.SetValue(c.history[c.histIdx])
			} else {
				c.histIdx = -1
				c.textarea.SetValue(c.draft)
			}
			return c, nil
		}
	}

	var cmd tea.Cmd
	c.textarea, cmd = c.textarea.Update(msg)
	return c, cmd
}

// View renders the input area.
func (c CommandInput) View() string {
	return c.textarea.View()
}
