package tui

import (
	"time"

	wsclient "github.com/dohr-michael/ozzie/clients/ws"
	"github.com/dohr-michael/ozzie/internal/events"
)

// StreamStartMsg signals the beginning of a streaming response.
type StreamStartMsg struct{}

// StreamDeltaMsg carries an incremental text chunk.
type StreamDeltaMsg struct {
	Content string
	Index   int
}

// StreamEndMsg signals the end of streaming.
type StreamEndMsg struct{}

// AssistantMessageMsg carries a complete (non-streamed) assistant response.
type AssistantMessageMsg struct {
	Content string
	Error   string
}

// ToolCallMsg represents a tool call lifecycle event.
type ToolCallMsg struct {
	Status    string
	Name      string
	Arguments map[string]any
	Result    string
	Error     string
}

// PromptRequestMsg asks the user for interactive input.
type PromptRequestMsg struct {
	Type        string
	Label       string
	Options     []events.PromptOption
	Token       string
	HelpText    string
	Placeholder string
	MinSelect   int
	MaxSelect   int
}

// ConnectedMsg signals a successful WS connection (or reconnection).
type ConnectedMsg struct {
	SessionID string
	Client    *wsclient.Client // non-nil on reconnection
}

// DisconnectedMsg signals a lost WS connection.
type DisconnectedMsg struct {
	Err error
}

// UserSubmitMsg carries text submitted by the user input.
type UserSubmitMsg struct {
	Content string
}

// SkillStartedMsg signals the start of a skill execution.
type SkillStartedMsg struct {
	Name string
}

// SkillCompletedMsg signals the end of a skill execution.
type SkillCompletedMsg struct {
	Name     string
	Error    string
	Duration time.Duration
}

// SkillStepStartedMsg signals the start of a skill step.
type SkillStepStartedMsg struct {
	SkillName string
	StepID    string
	StepTitle string
}

// SkillStepCompletedMsg signals the end of a skill step.
type SkillStepCompletedMsg struct {
	SkillName string
	StepID    string
	Error     string
	Duration  time.Duration
}

// LLMTelemetryMsg carries token usage from an internal.llm.call event.
type LLMTelemetryMsg struct {
	Model     string
	TokensIn  int
	TokensOut int
}

// sendErrorMsg carries an error from an async WS send.
type sendErrorMsg struct {
	err error
}
