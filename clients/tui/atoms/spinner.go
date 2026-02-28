// Package atoms provides low-level TUI building blocks.
package atoms

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Spinner wraps bubbles/spinner with a configurable style.
type Spinner struct {
	Model spinner.Model
}

// NewSpinner creates a spinner with the dots pattern.
func NewSpinner(color lipgloss.AdaptiveColor) Spinner {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(color)
	return Spinner{Model: s}
}

// Init returns the spinner tick command.
func (s Spinner) Init() tea.Cmd {
	return s.Model.Tick
}

// Update handles spinner messages.
func (s Spinner) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	var cmd tea.Cmd
	s.Model, cmd = s.Model.Update(msg)
	return s, cmd
}

// View renders the spinner frame.
func (s Spinner) View() string {
	return s.Model.View()
}
