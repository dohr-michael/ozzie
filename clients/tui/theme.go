// Package tui provides a terminal user interface for the Ozzie gateway.
package tui

import "github.com/charmbracelet/lipgloss"

// Adaptive colors (light/dark terminal detection).
var (
	ColorUser      = lipgloss.AdaptiveColor{Light: "#0070F3", Dark: "#79C0FF"}
	ColorAssistant = lipgloss.AdaptiveColor{Light: "#6B21A8", Dark: "#D8A6FF"}
	ColorTool      = lipgloss.AdaptiveColor{Light: "#065F46", Dark: "#7EE2B8"}
	ColorError     = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#FF6B6B"}
	ColorMuted     = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	ColorStatusBg  = lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#1F2937"}
	ColorStatusFg  = lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"}
	ColorBorder    = lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}
)

// Component styles.
var (
	UserStyle = lipgloss.NewStyle().
			Foreground(ColorUser).
			Bold(true)

	AssistantStyle = lipgloss.NewStyle().
			Foreground(ColorAssistant).
			Bold(true)

	ToolStyle = lipgloss.NewStyle().
			Foreground(ColorTool)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StatusBarStyle = lipgloss.NewStyle().
			Background(ColorStatusBg).
			Foreground(ColorStatusFg).
			Padding(0, 1)

	ToolBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	PromptBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorAssistant).
				Padding(0, 1)
)
