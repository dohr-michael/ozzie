package molecules

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	toolNameStyle = lipgloss.NewStyle().Bold(true)
	checkStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#065F46", Dark: "#7EE2B8"})
	failStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#FF6B6B"})
	resultStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"})
)

// ToolHeader renders a tool call header line:
//
//	"⠋ tool_name(args...)" | "✓ tool_name(args...) → result" | "✗ tool_name: error"
func ToolHeader(status, name, spinnerView string, args map[string]any, result, errMsg string) string {
	switch status {
	case "started":
		argsPreview := truncateArgs(args, 60)
		return fmt.Sprintf("%s %s(%s)", spinnerView, toolNameStyle.Render(name), argsPreview)
	case "completed":
		argsPreview := truncateArgs(args, 40)
		header := fmt.Sprintf("%s %s(%s)", checkStyle.Render("✓"), toolNameStyle.Render(name), argsPreview)
		if result != "" {
			preview := truncateResult(result, 60)
			header += resultStyle.Render(" → "+preview)
		}
		return header
	case "failed":
		msg := errMsg
		if len(msg) > 80 {
			msg = msg[:77] + "..."
		}
		return fmt.Sprintf("%s %s: %s", failStyle.Render("✗"), toolNameStyle.Render(name), msg)
	default:
		return toolNameStyle.Render(name)
	}
}

func truncateArgs(args map[string]any, max int) string {
	if len(args) == 0 {
		return ""
	}
	s := fmt.Sprintf("%v", args)
	// Strip outer "map[" and "]"
	if len(s) > 4 && s[:4] == "map[" {
		s = s[4 : len(s)-1]
	}
	if len(s) > max {
		s = s[:max-3] + "..."
	}
	return s
}

func truncateResult(result string, max int) string {
	// Take only the first line.
	if idx := strings.IndexByte(result, '\n'); idx >= 0 {
		result = result[:idx]
	}
	result = strings.TrimSpace(result)
	if len(result) > max {
		result = result[:max-3] + "..."
	}
	return result
}
