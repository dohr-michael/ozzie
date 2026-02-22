package events

import (
	"encoding/json"
	"time"
)

// EventPayload is the interface all typed payloads implement.
type EventPayload interface {
	EventType() EventType
}

// =============================================================================
// USER EVENTS
// =============================================================================

type UserMessagePayload struct {
	Content string `json:"content"`
}

func (UserMessagePayload) EventType() EventType { return EventUserMessage }

// =============================================================================
// ASSISTANT EVENTS
// =============================================================================

type StreamPhase string

const (
	StreamPhaseStart StreamPhase = "start"
	StreamPhaseDelta StreamPhase = "delta"
	StreamPhaseEnd   StreamPhase = "end"
)

type AssistantStreamPayload struct {
	Phase   StreamPhase `json:"phase"`
	Content string      `json:"content"`
	Index   int         `json:"index"`
}

func (AssistantStreamPayload) EventType() EventType { return EventAssistantStream }

type AssistantMessagePayload struct {
	Content string         `json:"content"`
	Error   string         `json:"error,omitempty"`
	Context map[string]any `json:"context,omitempty"`
}

func (AssistantMessagePayload) EventType() EventType { return EventAssistantMessage }

// =============================================================================
// TOOL EVENTS
// =============================================================================

type ToolStatus string

const (
	ToolStatusStarted   ToolStatus = "started"
	ToolStatusCompleted ToolStatus = "completed"
	ToolStatusFailed    ToolStatus = "failed"
)

type ToolCallPayload struct {
	Status    ToolStatus     `json:"status"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Result    string         `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
}

func (ToolCallPayload) EventType() EventType { return EventToolCall }

// =============================================================================
// PROMPT EVENTS
// =============================================================================

type PromptRequestPayload struct {
	Type        PromptType     `json:"type"`
	Label       string         `json:"label"`
	Options     []PromptOption `json:"options,omitempty"`
	Required    bool           `json:"required"`
	Default     any            `json:"default,omitempty"`
	Validation  string         `json:"validation,omitempty"`
	Placeholder string         `json:"placeholder,omitempty"`
	Token       string         `json:"token"`
	Context     map[string]any `json:"context,omitempty"`
}

func (PromptRequestPayload) EventType() EventType { return EventPromptRequest }

type PromptResponsePayload struct {
	Value     any    `json:"value"`
	Cancelled bool   `json:"cancelled"`
	Token     string `json:"token"`
}

func (PromptResponsePayload) EventType() EventType { return EventPromptResponse }

// =============================================================================
// INTERNAL EVENTS
// =============================================================================

type LLMCallPayload struct {
	Phase        string        `json:"phase"`
	Model        string        `json:"model"`
	Provider     string        `json:"provider,omitempty"`
	MessageCount int           `json:"message_count,omitempty"`
	TokensInput  int           `json:"tokens_input,omitempty"`
	TokensOutput int           `json:"tokens_output,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
	Error        string        `json:"error,omitempty"`
}

func (LLMCallPayload) EventType() EventType { return EventLLMCall }

// =============================================================================
// TYPED EVENT CONSTRUCTORS
// =============================================================================

func NewTypedEvent(source EventSource, payload EventPayload) Event {
	return Event{
		ID:        generateEventID(),
		Type:      payload.EventType(),
		Timestamp: time.Now(),
		Source:    source,
		Payload:   toMap(payload),
	}
}

func NewTypedEventWithSession(source EventSource, payload EventPayload, sessionID string) Event {
	return Event{
		ID:        generateEventID(),
		SessionID: sessionID,
		Type:      payload.EventType(),
		Timestamp: time.Now(),
		Source:    source,
		Payload:   toMap(payload),
	}
}

func toMap(v any) map[string]any {
	var result map[string]any
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

// =============================================================================
// TYPED PAYLOAD EXTRACTORS
// =============================================================================

func ExtractPayload[T EventPayload](e Event) (T, bool) {
	var result T
	data, err := json.Marshal(e.Payload)
	if err != nil {
		return result, false
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, false
	}
	return result, true
}

func GetUserMessagePayload(e Event) (UserMessagePayload, bool) {
	return ExtractPayload[UserMessagePayload](e)
}

func GetAssistantStreamPayload(e Event) (AssistantStreamPayload, bool) {
	return ExtractPayload[AssistantStreamPayload](e)
}

func GetAssistantMessagePayload(e Event) (AssistantMessagePayload, bool) {
	return ExtractPayload[AssistantMessagePayload](e)
}

func GetToolCallPayload(e Event) (ToolCallPayload, bool) {
	return ExtractPayload[ToolCallPayload](e)
}

func GetPromptRequestPayload(e Event) (PromptRequestPayload, bool) {
	return ExtractPayload[PromptRequestPayload](e)
}

func GetPromptResponsePayload(e Event) (PromptResponsePayload, bool) {
	return ExtractPayload[PromptResponsePayload](e)
}

func GetLLMCallPayload(e Event) (LLMCallPayload, bool) {
	return ExtractPayload[LLMCallPayload](e)
}
