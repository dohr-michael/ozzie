package organisms

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// InformationPanel displays the status bar with session, model, tokens, connection, and mode.
type InformationPanel struct {
	sessionID string
	model     string
	tokensIn  int
	tokensOut int
	connected bool
	connErr   error
	mode      Mode
	width     int
	style     lipgloss.Style
}

// NewInformationPanel creates a new status bar panel.
func NewInformationPanel(style lipgloss.Style) InformationPanel {
	return InformationPanel{
		style:     style,
		connected: true,
	}
}

// Setters

// SetSession updates the session ID.
func (p *InformationPanel) SetSession(id string) { p.sessionID = id }

// SetModel updates the model name.
func (p *InformationPanel) SetModel(model string) { p.model = model }

// AddTokens accumulates token usage.
func (p *InformationPanel) AddTokens(in, out int) { p.tokensIn += in; p.tokensOut += out }

// SetConnected updates the connection state.
func (p *InformationPanel) SetConnected(connected bool, err error) {
	p.connected = connected
	p.connErr = err
}

// SetMode updates the displayed interaction mode.
func (p *InformationPanel) SetMode(mode Mode) { p.mode = mode }

// SetWidth updates the rendering width.
func (p *InformationPanel) SetWidth(w int) { p.width = w }

// Getters

// SessionID returns the session ID.
func (p *InformationPanel) SessionID() string { return p.sessionID }

// Model returns the model name.
func (p *InformationPanel) Model() string { return p.model }

// TokensIn returns accumulated input tokens.
func (p *InformationPanel) TokensIn() int { return p.tokensIn }

// TokensOut returns accumulated output tokens.
func (p *InformationPanel) TokensOut() int { return p.tokensOut }

// Connected returns whether the WS is connected.
func (p *InformationPanel) Connected() bool { return p.connected }

// ConnErr returns the last connection error.
func (p *InformationPanel) ConnErr() error { return p.connErr }

// View renders the status bar.
func (p InformationPanel) View() string {
	sid := p.sessionID
	if len(sid) > 8 {
		sid = sid[:8]
	}

	connStatus := "connected"
	if !p.connected {
		connStatus = "disconnected"
	}

	modeStr := ""
	switch p.mode {
	case ModeStreaming:
		modeStr = " | streaming"
	case ModePrompting:
		modeStr = " | prompting"
	}

	tokenStr := ""
	if p.tokensIn > 0 || p.tokensOut > 0 {
		tokenStr = fmt.Sprintf(" | %s in / %s out", formatTokens(p.tokensIn), formatTokens(p.tokensOut))
	}

	modelStr := ""
	if p.model != "" {
		modelStr = " | " + p.model
	}

	bar := fmt.Sprintf(" sess:%s%s%s%s | %s ", sid, modelStr, tokenStr, modeStr, connStatus)
	return p.style.Width(p.width).Render(bar)
}

func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000.0)
	}
	return fmt.Sprintf("%d", n)
}
