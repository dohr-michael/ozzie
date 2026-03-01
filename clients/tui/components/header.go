package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Header displays LLM context and token usage.
type Header struct {
	width      int
	provider   string // e.g., "ollama", "anthropic", "openai"
	model      string // e.g., "llama3.1", "claude-sonnet"
	endpoint   string // e.g., "localhost:11434" (for ollama)
	tokensUsed int
	streaming  bool
}

// NewHeader creates a new header component.
func NewHeader() *Header {
	return &Header{}
}

// SetProvider sets the LLM provider info.
func (h *Header) SetProvider(provider, model, endpoint string) {
	h.provider = provider
	h.model = model
	h.endpoint = endpoint
}

// SetTokens sets the token count.
func (h *Header) SetTokens(tokens int) {
	h.tokensUsed = tokens
}

// AddTokens adds to the token count.
func (h *Header) AddTokens(tokens int) {
	h.tokensUsed += tokens
}

// SetStreaming sets the streaming indicator.
func (h *Header) SetStreaming(streaming bool) {
	h.streaming = streaming
}

// SetWidth sets the component width.
func (h *Header) SetWidth(width int) {
	h.width = width
}

// Update handles messages.
func (h *Header) Update(msg tea.Msg) (*Header, tea.Cmd) {
	switch msg := msg.(type) {
	case HeaderSetStreamingMsg:
		h.streaming = msg.Streaming
	case HeaderAddTokensMsg:
		h.tokensUsed += msg.Tokens
	case HeaderSetSizeMsg:
		h.width = msg.Width
	}
	return h, nil
}

// View renders the header.
func (h *Header) View() string {
	// Build left side: provider | model | endpoint
	var left strings.Builder
	left.WriteString(HeaderProviderStyle.Render(h.provider))
	if h.model != "" {
		left.WriteString(" | ")
		left.WriteString(HeaderModelStyle.Render(h.model))
	}
	if h.endpoint != "" {
		left.WriteString(" | ")
		left.WriteString(HeaderEndpointStyle.Render(h.endpoint))
	}

	// Build right side: streaming indicator + tokens
	var right strings.Builder
	if h.streaming {
		right.WriteString(HeaderStreamStyle.Render("● "))
	}
	if h.tokensUsed > 0 {
		right.WriteString(HeaderTokenStyle.Render(FormatTokenCount(h.tokensUsed)))
	}

	leftStr := left.String()
	rightStr := right.String()

	// Calculate padding
	leftWidth := len(h.provider) + len(h.model) + len(h.endpoint) + 6 // rough estimate
	rightWidth := len(rightStr)
	padding := h.width - leftWidth - rightWidth - 2 // -2 for padding
	if padding < 1 {
		padding = 1
	}

	content := leftStr + strings.Repeat(" ", padding) + rightStr
	return HeaderStyle.Width(h.width).Render(content)
}
