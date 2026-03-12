// Package sessions provides session management for Ozzie.
package sessions

import (
	"time"

	"github.com/dohr-michael/ozzie/internal/brain"
	"github.com/dohr-michael/ozzie/internal/core/events"
)

// SessionStatus represents the lifecycle state of a session.
type SessionStatus string

const (
	SessionActive SessionStatus = "active"
	SessionClosed SessionStatus = "closed"
)

// RoleToolLog is a message role for persisted tool call summaries.
// These are displayed in TUI history but filtered out before sending to the LLM.
const RoleToolLog = "tool_log"

// TokenUsage tracks cumulative token consumption.
// Canonical definition lives in brain.TokenUsage.
type TokenUsage = brain.TokenUsage

// Session holds metadata about a conversation session.
type Session struct {
	ID              string                            `json:"id"`
	Title           string                            `json:"title"`
	CreatedAt       time.Time                         `json:"created_at"`
	UpdatedAt       time.Time                         `json:"updated_at"`
	Status          SessionStatus                     `json:"status"`
	Model           string                            `json:"model,omitempty"`
	MessageCount    int                               `json:"message_count"`
	TokenUsage      TokenUsage                        `json:"token_usage"`
	RootDir         string                            `json:"root_dir,omitempty"`
	Language        string                            `json:"language,omitempty"`
	Summary         string                            `json:"summary,omitempty"`       // compressed context from older messages
	SummaryUpTo     int                               `json:"summary_up_to,omitempty"` // index (exclusive) of last summarized message
	Metadata        map[string]string                 `json:"metadata,omitempty"`
	ApprovedTools   []string                          `json:"approved_tools,omitempty"`   // dangerous tools approved for this session
	ToolConstraints map[string]*events.ToolConstraint `json:"tool_constraints,omitempty"` // per-tool argument constraints
	PolicyName      string                            `json:"policy_name,omitempty"`      // policy applied to this session
}

// Message is a single turn in a conversation, serializable to JSONL.
type Message struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	Ts      time.Time `json:"ts"`
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
