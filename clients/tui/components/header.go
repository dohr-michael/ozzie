package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Header displays LLM context and token usage as a compact footer.
type Header struct {
	width      int
	provider   string // e.g., "ollama", "anthropic", "openai"
	model      string // e.g., "llama3.1", "claude-sonnet"
	endpoint   string // kept for API compat but not rendered
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

// View renders the footer as a compact single-line bar.
// Format: model │ 12 345 tokens │ ● streaming
func (h *Header) View() string {
	sep := FooterSeparatorStyle.Render(" │ ")

	var segments []string

	// Model name (or provider as fallback)
	name := h.model
	if name == "" {
		name = h.provider
	}
	if name != "" {
		segments = append(segments, HeaderModelStyle.Render(name))
	}

	// Token count
	if h.tokensUsed > 0 {
		segments = append(segments, HeaderTokenStyle.Render(FormatTokenCount(h.tokensUsed)+" tokens"))
	}

	// Streaming indicator
	if h.streaming {
		segments = append(segments, HeaderStreamStyle.Render("● streaming"))
	}

	content := strings.Join(segments, sep)
	return HeaderStyle.Width(h.width).Render(content)
}
