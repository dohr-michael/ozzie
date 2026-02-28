package organisms

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dohr-michael/ozzie/clients/tui/atoms"
	"github.com/dohr-michael/ozzie/clients/tui/molecules"
)

// ToolBlock represents a tool call in the conversation history.
type ToolBlock struct {
	name      string
	status    string
	args      map[string]any
	result    string
	errMsg    string
	spinner   atoms.Spinner
	collapsed bool
	style     lipgloss.Style

	cached string // rendered view cache
}

// NewToolBlock creates a tool block from a started tool call.
func NewToolBlock(name string, args map[string]any, style lipgloss.Style) *ToolBlock {
	return &ToolBlock{
		name:      name,
		status:    "started",
		args:      args,
		spinner:   atoms.NewSpinner(lipgloss.AdaptiveColor{Light: "#065F46", Dark: "#7EE2B8"}),
		collapsed: true,
		style:     style,
	}
}

// UpdateStatus updates the tool block with completion or failure.
func (tb *ToolBlock) UpdateStatus(status, result, errMsg string) {
	tb.status = status
	tb.result = result
	tb.errMsg = errMsg
	tb.cached = "" // invalidate
}

// ToggleCollapsed toggles the collapsed state.
func (tb *ToolBlock) ToggleCollapsed() {
	tb.collapsed = !tb.collapsed
	tb.cached = "" // invalidate
}

// Name returns the tool name.
func (tb *ToolBlock) Name() string {
	return tb.name
}

// IsComplete returns whether the tool call is done.
func (tb *ToolBlock) IsComplete() bool {
	return tb.status == "completed" || tb.status == "failed"
}

// View renders the tool block.
func (tb *ToolBlock) View() string {
	// Return cached output for completed blocks whose collapsed state hasn't changed.
	if tb.IsComplete() && tb.cached != "" {
		return tb.cached
	}

	header := molecules.ToolHeader(tb.status, tb.name, tb.spinner.View(), tb.args, tb.result, tb.errMsg)

	if tb.collapsed {
		result := tb.style.Render(header)
		if tb.IsComplete() {
			tb.cached = result
		}
		return result
	}

	// Expanded view: show full args and result.
	var sb strings.Builder
	sb.WriteString(header)

	if len(tb.args) > 0 {
		argsJSON, err := json.MarshalIndent(tb.args, "  ", "  ")
		if err == nil {
			sb.WriteString("\n  Args: " + string(argsJSON))
		}
	}

	if tb.result != "" {
		result := tb.result
		if len(result) > 500 {
			result = result[:497] + "..."
		}
		sb.WriteString(fmt.Sprintf("\n  Result: %s", result))
	}

	if tb.errMsg != "" {
		sb.WriteString(fmt.Sprintf("\n  Error: %s", tb.errMsg))
	}

	result := tb.style.Render(sb.String())
	if tb.IsComplete() {
		tb.cached = result
	}
	return result
}
