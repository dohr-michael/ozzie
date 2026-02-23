// Package sessions provides session management for Ozzie.
package sessions

import (
	"time"

	"github.com/cloudwego/eino/schema"
)

// SessionStatus represents the lifecycle state of a session.
type SessionStatus string

const (
	SessionActive SessionStatus = "active"
	SessionClosed SessionStatus = "closed"
)

// TokenUsage tracks cumulative token consumption for a session.
type TokenUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// Session holds metadata about a conversation session.
type Session struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Status       SessionStatus     `json:"status"`
	Model        string            `json:"model,omitempty"`
	MessageCount int               `json:"message_count"`
	TokenUsage   TokenUsage        `json:"token_usage"`
	RootDir      string            `json:"root_dir,omitempty"`
	Language     string            `json:"language,omitempty"`
	Summary      string            `json:"summary,omitempty"`        // compressed context from older messages
	SummaryUpTo  int               `json:"summary_up_to,omitempty"`  // index (exclusive) of last summarized message
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Message is a single turn in a conversation, serializable to JSONL.
type Message struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	Ts      time.Time `json:"ts"`
}

// ToSchemaMessage converts a session Message to an Eino schema.Message.
func (m Message) ToSchemaMessage() *schema.Message {
	return &schema.Message{
		Role:    schema.RoleType(m.Role),
		Content: m.Content,
	}
}

// NewMessageFromSchema converts an Eino schema.Message to a session Message.
func NewMessageFromSchema(msg *schema.Message) Message {
	return Message{
		Role:    string(msg.Role),
		Content: msg.Content,
		Ts:      time.Now(),
	}
}

// Store defines the persistence interface for sessions.
type Store interface {
	Create() (*Session, error)
	Get(id string) (*Session, error)
	List() ([]*Session, error)
	UpdateMeta(s *Session) error
	Close(id string) error
	AppendMessage(sessionID string, msg Message) error
	LoadMessages(sessionID string) ([]Message, error)
}
