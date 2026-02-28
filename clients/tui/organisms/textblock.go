// Package organisms provides high-level TUI components.
package organisms

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/dohr-michael/ozzie/clients/tui/atoms"
)

// TextBlock accumulates streamed text and renders it.
type TextBlock struct {
	role     string
	style    lipgloss.Style
	content  strings.Builder
	complete bool
	spinner  atoms.Spinner
	width    int
	cached   string // rendered view cache (set once on completion)
}

// NewTextBlock creates a text block for a given role.
func NewTextBlock(role string, style lipgloss.Style, width int) *TextBlock {
	return &TextBlock{
		role:    role,
		style:   style,
		spinner: atoms.NewSpinner(lipgloss.AdaptiveColor{Light: "#6B21A8", Dark: "#D8A6FF"}),
		width:   width,
	}
}

// AppendDelta adds a streaming content chunk.
func (tb *TextBlock) AppendDelta(content string) {
	tb.content.WriteString(content)
}

// SetComplete marks the block as finished.
func (tb *TextBlock) SetComplete() {
	tb.complete = true
}

// IsComplete returns whether the block is finalized.
func (tb *TextBlock) IsComplete() bool {
	return tb.complete
}

// Content returns the accumulated text.
func (tb *TextBlock) Content() string {
	return tb.content.String()
}

// Role returns the block's role.
func (tb *TextBlock) Role() string {
	return tb.role
}

// SetWidth updates the rendering width.
func (tb *TextBlock) SetWidth(w int) {
	tb.width = w
}

// View renders the text block with role label.
func (tb *TextBlock) View() string {
	// Return cached output for completed blocks (avoids re-rendering markdown).
	if tb.complete && tb.cached != "" {
		return tb.cached
	}

	label := tb.style.Render(tb.role)
	text := tb.content.String()

	if !tb.complete && text == "" {
		return label + " " + tb.spinner.View()
	}

	rendered := tb.renderMarkdown(text)

	if !tb.complete {
		return label + " " + rendered + tb.spinner.View()
	}

	result := label + " " + rendered
	tb.cached = result
	return result
}

// renderMarkdown renders text as markdown when complete, plain during streaming.
func (tb *TextBlock) renderMarkdown(text string) string {
	if !tb.complete {
		return text
	}

	w := tb.width - 6 // account for label + padding
	if w < 20 {
		w = 20
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(w),
	)
	if err != nil {
		return text
	}

	out, err := r.Render(text)
	if err != nil {
		return text
	}

	return strings.TrimSpace(out)
}
