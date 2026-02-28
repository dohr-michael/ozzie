package atoms

import "github.com/charmbracelet/lipgloss"

// StyledLabel renders a role label (e.g. "You", "Ozzie", "Tool") with the given style.
func StyledLabel(role string, style lipgloss.Style) string {
	return style.Render(role)
}
