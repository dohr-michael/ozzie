// Package connector defines types for external platform connections (Discord, Slack, etc.).
package connector

// Identity represents an external user on a platform.
type Identity struct {
	Platform  string // "discord", "whatsapp", "web", "tui"
	UserID    string // platform-specific user ID
	Name      string // display name
	ServerID  string // guild/workspace ID (empty for DMs)
	ChannelID string // channel/conversation ID
}
