package atoms

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CaretBlinkMsg toggles the caret visibility.
type CaretBlinkMsg struct{}

// Caret is a blinking block cursor for the streaming indicator.
type Caret struct {
	Visible bool
	style   lipgloss.Style
}

// NewCaret creates a blinking caret.
func NewCaret(color lipgloss.AdaptiveColor) Caret {
	return Caret{
		Visible: true,
		style:   lipgloss.NewStyle().Foreground(color),
	}
}

// BlinkCmd returns a command that sends a blink message after a delay.
func BlinkCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return CaretBlinkMsg{}
	})
}

// Update toggles visibility on blink messages.
func (c Caret) Update(msg tea.Msg) (Caret, tea.Cmd) {
	if _, ok := msg.(CaretBlinkMsg); ok {
		c.Visible = !c.Visible
		return c, BlinkCmd()
	}
	return c, nil
}

// View renders the caret.
func (c Caret) View() string {
	if c.Visible {
		return c.style.Render("\u2588") // full block
	}
	return " "
}
