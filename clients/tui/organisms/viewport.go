package organisms

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ContentBlock is the interface for renderable conversation blocks.
type ContentBlock interface {
	View() string
	IsComplete() bool
}

// OutputViewport manages the scrollable conversation history.
type OutputViewport struct {
	viewport viewport.Model
	blocks   []ContentBlock
	width    int
	height   int
}

// NewOutputViewport creates a viewport for the conversation history.
func NewOutputViewport(width, height int) OutputViewport {
	vp := viewport.New(width, height)
	vp.SetContent("")
	// Disable ALL built-in keybindings to prevent conflict with the textarea.
	// Scroll is handled explicitly via PageUp/PageDown in the main model.
	vp.KeyMap = viewport.KeyMap{}
	vp.MouseWheelEnabled = false
	return OutputViewport{
		viewport: vp,
		width:    width,
		height:   height,
	}
}

// SetSize updates the viewport dimensions.
func (o *OutputViewport) SetSize(width, height int) {
	o.width = width
	o.height = height
	o.viewport.Width = width
	o.viewport.Height = height
	o.refresh()
}

// AppendBlock adds a new content block and re-renders.
func (o *OutputViewport) AppendBlock(block ContentBlock) {
	o.blocks = append(o.blocks, block)
	o.refresh()
}

// BlockCount returns the number of blocks.
func (o *OutputViewport) BlockCount() int {
	return len(o.blocks)
}

// LastBlock returns the last block, or nil.
func (o *OutputViewport) LastBlock() ContentBlock {
	if len(o.blocks) == 0 {
		return nil
	}
	return o.blocks[len(o.blocks)-1]
}

// Block returns the block at position i, or nil if out of range.
func (o *OutputViewport) Block(i int) ContentBlock {
	if i < 0 || i >= len(o.blocks) {
		return nil
	}
	return o.blocks[i]
}

// PageUp scrolls up by one page.
func (o *OutputViewport) PageUp() {
	o.viewport.PageUp()
}

// PageDown scrolls down by one page.
func (o *OutputViewport) PageDown() {
	o.viewport.PageDown()
}

// Refresh re-renders all blocks into the viewport and scrolls to the bottom.
func (o *OutputViewport) Refresh() {
	o.refresh()
}

func (o *OutputViewport) refresh() {
	var sb strings.Builder
	for i, block := range o.blocks {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(block.View())
	}
	o.viewport.SetContent(sb.String())
	o.viewport.GotoBottom()
}

// Update handles viewport messages.
func (o OutputViewport) Update(msg tea.Msg) (OutputViewport, tea.Cmd) {
	var cmd tea.Cmd
	o.viewport, cmd = o.viewport.Update(msg)
	return o, cmd
}

// View renders the viewport.
func (o OutputViewport) View() string {
	return o.viewport.View()
}
