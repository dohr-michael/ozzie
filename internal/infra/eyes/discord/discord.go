// Package discord provides a Discord connector for Ozzie.
package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/dohr-michael/ozzie/pkg/connector"
)

// Config configures the Discord connector.
type Config struct {
	Token        string
	AdminChannel string // channel ID for admin notifications
}

// Connector implements connector.Connector for Discord.
type Connector struct {
	token        string
	adminChannel string

	session *discordgo.Session
	handler connector.IncomingHandler
	ready   chan struct{}

	mu     sync.Mutex
	closed bool
}

var _ connector.Connector = (*Connector)(nil)
var _ connector.Reactioner = (*Connector)(nil)

// New creates a new Discord connector.
func New(cfg Config) (*Connector, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("discord: token is required")
	}
	return &Connector{
		token:        cfg.Token,
		adminChannel: cfg.AdminChannel,
		ready:        make(chan struct{}),
	}, nil
}

// Name returns the connector name.
func (c *Connector) Name() string { return "discord" }

// Start connects to Discord and blocks until ctx is cancelled.
func (c *Connector) Start(ctx context.Context, handler connector.IncomingHandler) error {
	c.handler = handler

	dg, err := discordgo.New("Bot " + c.token)
	if err != nil {
		return fmt.Errorf("discord: create session: %w", err)
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	dg.AddHandler(c.onReady)
	dg.AddHandler(c.onMessageCreate)

	if err := dg.Open(); err != nil {
		return fmt.Errorf("discord: open: %w", err)
	}
	c.mu.Lock()
	c.session = dg
	c.closed = false
	c.mu.Unlock()

	slog.Info("discord connector connected", "user", dg.State.User.Username)

	// Block until context is cancelled
	<-ctx.Done()
	return dg.Close()
}

// Send sends a message to a Discord channel, splitting if >2000 chars.
func (c *Connector) Send(_ context.Context, msg connector.OutgoingMessage) error {
	c.mu.Lock()
	s := c.session
	c.mu.Unlock()
	if s == nil {
		return fmt.Errorf("discord: not connected")
	}

	// If replying, use the message reference
	var ref *discordgo.MessageReference
	if msg.ReplyToID != "" {
		ref = &discordgo.MessageReference{MessageID: msg.ReplyToID}
	}

	for _, chunk := range splitMessage(msg.Content, 2000) {
		data := &discordgo.MessageSend{
			Content:   chunk,
			Reference: ref,
		}
		if _, err := s.ChannelMessageSendComplex(msg.ChannelID, data); err != nil {
			return fmt.Errorf("discord: send: %w", err)
		}
		ref = nil // only reply on first chunk
	}
	return nil
}

// AddReaction adds a progress reaction to a message.
func (c *Connector) AddReaction(_ context.Context, channelID, messageID string, reaction connector.ReactionType) error {
	c.mu.Lock()
	s := c.session
	c.mu.Unlock()
	if s == nil {
		return fmt.Errorf("discord: not connected")
	}
	return s.MessageReactionAdd(channelID, messageID, reactionEmoji(reaction))
}

// RemoveReaction removes the bot's progress reaction from a message.
func (c *Connector) RemoveReaction(_ context.Context, channelID, messageID string, reaction connector.ReactionType) error {
	c.mu.Lock()
	s := c.session
	c.mu.Unlock()
	if s == nil {
		return fmt.Errorf("discord: not connected")
	}
	return s.MessageReactionRemove(channelID, messageID, reactionEmoji(reaction), s.State.User.ID)
}

// reactionEmoji maps a ReactionType to a Discord emoji.
func reactionEmoji(r connector.ReactionType) string {
	switch r {
	case connector.ReactionThinking:
		return "\U0001f9e0" // 🧠
	case connector.ReactionWeb:
		return "\U0001f310" // 🌐
	case connector.ReactionCommand:
		return "\U0001f4bb" // 💻
	case connector.ReactionEdit:
		return "\u270f\ufe0f" // ✏️
	case connector.ReactionTask:
		return "\U0001f4cb" // 📋
	case connector.ReactionMemory:
		return "\U0001f4ad" // 💭
	case connector.ReactionSchedule:
		return "\u23f0" // ⏰
	case connector.ReactionActivate:
		return "\U0001f50c" // 🔌
	default:
		return "\U0001f527" // 🔧
	}
}

// SendTyping starts a typing indicator that refreshes every 8 seconds.
// Returns a cancel function to stop the indicator.
func (c *Connector) SendTyping(_ context.Context, channelID string) func() {
	c.mu.Lock()
	s := c.session
	c.mu.Unlock()
	if s == nil {
		return func() {}
	}

	done := make(chan struct{})
	var once sync.Once

	go func() {
		_ = s.ChannelTyping(channelID)
		ticker := time.NewTicker(8 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = s.ChannelTyping(channelID)
			case <-done:
				return
			}
		}
	}()

	return func() {
		once.Do(func() { close(done) })
	}
}

// Stop disconnects from Discord.
func (c *Connector) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil && !c.closed {
		c.closed = true
		return c.session.Close()
	}
	return nil
}

// SendAdmin sends a message to the admin channel.
func (c *Connector) SendAdmin(_ context.Context, content string) error {
	if c.adminChannel == "" {
		return fmt.Errorf("discord: no admin channel configured")
	}
	c.mu.Lock()
	s := c.session
	c.mu.Unlock()
	if s == nil {
		return fmt.Errorf("discord: not connected")
	}
	_, err := s.ChannelMessageSend(c.adminChannel, content)
	return err
}

// AdminChannel returns the configured admin channel ID.
func (c *Connector) AdminChannel() string {
	return c.adminChannel
}

func (c *Connector) onReady(_ *discordgo.Session, _ *discordgo.Ready) {
	select {
	case <-c.ready:
	default:
		close(c.ready)
	}
}

func (c *Connector) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from bots (including ourselves)
	if m.Author.Bot {
		return
	}

	handler := c.handler
	if handler == nil {
		return
	}

	guildID := m.GuildID // empty for DMs

	handler(context.Background(), connector.IncomingMessage{
		Identity: connector.Identity{
			Platform:  "discord",
			UserID:    m.Author.ID,
			Name:      m.Author.Username,
			ServerID:  guildID,
			ChannelID: m.ChannelID,
		},
		Content:   m.Content,
		ChannelID: m.ChannelID,
		MessageID: m.ID,
		Timestamp: m.Timestamp,
	})
}

// splitMessage splits a message into chunks of at most maxLen characters.
// It tries to split at newlines, then spaces, and falls back to hard cuts.
func splitMessage(s string, maxLen int) []string {
	if len(s) == 0 {
		return []string{""}
	}
	if len(s) <= maxLen {
		return []string{s}
	}

	var chunks []string
	for len(s) > 0 {
		if len(s) <= maxLen {
			chunks = append(chunks, s)
			break
		}

		chunk := s[:maxLen]

		// Try to find a split point
		splitIdx := -1

		// Prefer newline
		if idx := strings.LastIndex(chunk, "\n"); idx > 0 {
			splitIdx = idx
		}

		// Fall back to space
		if splitIdx < 0 {
			if idx := strings.LastIndex(chunk, " "); idx > 0 {
				splitIdx = idx
			}
		}

		if splitIdx > 0 {
			chunks = append(chunks, s[:splitIdx])
			s = s[splitIdx+1:] // skip the delimiter
		} else {
			// Hard cut
			chunks = append(chunks, chunk)
			s = s[maxLen:]
		}
	}
	return chunks
}
