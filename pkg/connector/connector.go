package connector

import (
	"context"
	"time"
)

// IncomingMessage represents a message received from an external platform.
type IncomingMessage struct {
	Identity  Identity
	Content   string
	ChannelID string
	MessageID string // platform-specific
	Timestamp time.Time
}

// OutgoingMessage represents a message to send to an external platform.
type OutgoingMessage struct {
	Content   string
	ChannelID string
	ReplyToID string // optional, for threading
}

// IncomingHandler is called when a connector receives a message.
type IncomingHandler func(ctx context.Context, msg IncomingMessage)

// Connector is the interface for external platform integrations.
type Connector interface {
	Name() string
	Start(ctx context.Context, handler IncomingHandler) error
	Send(ctx context.Context, msg OutgoingMessage) error
	SendTyping(ctx context.Context, channelID string) func() // returns cancel func
	Stop() error
}
