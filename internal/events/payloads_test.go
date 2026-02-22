package events

import (
	"testing"
	"time"
)

func TestTypedEvent_UserMessage(t *testing.T) {
	payload := UserMessagePayload{Content: "hello"}
	evt := NewTypedEvent(SourceAgent, payload)

	if evt.Type != EventUserMessage {
		t.Fatalf("expected type %q, got %q", EventUserMessage, evt.Type)
	}
	got, ok := ExtractPayload[UserMessagePayload](evt)
	if !ok {
		t.Fatal("ExtractPayload returned false")
	}
	if got.Content != "hello" {
		t.Fatalf("expected content %q, got %q", "hello", got.Content)
	}
}

func TestTypedEvent_AssistantStream(t *testing.T) {
	payload := AssistantStreamPayload{Phase: StreamPhaseDelta, Content: "chunk", Index: 3}
	evt := NewTypedEvent(SourceAgent, payload)

	if evt.Type != EventAssistantStream {
		t.Fatalf("expected type %q, got %q", EventAssistantStream, evt.Type)
	}
	got, ok := ExtractPayload[AssistantStreamPayload](evt)
	if !ok {
		t.Fatal("ExtractPayload returned false")
	}
	if got.Phase != StreamPhaseDelta {
		t.Fatalf("expected phase %q, got %q", StreamPhaseDelta, got.Phase)
	}
	if got.Content != "chunk" {
		t.Fatalf("expected content %q, got %q", "chunk", got.Content)
	}
	if got.Index != 3 {
		t.Fatalf("expected index 3, got %d", got.Index)
	}
}

func TestTypedEvent_AssistantMessage(t *testing.T) {
	payload := AssistantMessagePayload{
		Content: "response",
		Error:   "",
		Context: map[string]any{"key": "val"},
	}
	evt := NewTypedEvent(SourceAgent, payload)

	if evt.Type != EventAssistantMessage {
		t.Fatalf("expected type %q, got %q", EventAssistantMessage, evt.Type)
	}
	got, ok := ExtractPayload[AssistantMessagePayload](evt)
	if !ok {
		t.Fatal("ExtractPayload returned false")
	}
	if got.Content != "response" {
		t.Fatalf("expected content %q, got %q", "response", got.Content)
	}
}

func TestTypedEvent_ToolCall(t *testing.T) {
	payload := ToolCallPayload{
		Status:    ToolStatusCompleted,
		Name:      "search",
		Arguments: map[string]any{"query": "test"},
		Result:    "found 3 items",
	}
	evt := NewTypedEvent(SourceAgent, payload)

	if evt.Type != EventToolCall {
		t.Fatalf("expected type %q, got %q", EventToolCall, evt.Type)
	}
	got, ok := ExtractPayload[ToolCallPayload](evt)
	if !ok {
		t.Fatal("ExtractPayload returned false")
	}
	if got.Status != ToolStatusCompleted {
		t.Fatalf("expected status %q, got %q", ToolStatusCompleted, got.Status)
	}
	if got.Name != "search" {
		t.Fatalf("expected name %q, got %q", "search", got.Name)
	}
	if got.Result != "found 3 items" {
		t.Fatalf("expected result %q, got %q", "found 3 items", got.Result)
	}
}

func TestTypedEvent_LLMCall(t *testing.T) {
	payload := LLMCallPayload{
		Phase:        "response",
		Model:        "claude-sonnet",
		Provider:     "anthropic",
		MessageCount: 5,
		TokensInput:  100,
		TokensOutput: 50,
		Duration:     2 * time.Second,
	}
	evt := NewTypedEvent(SourceAgent, payload)

	if evt.Type != EventLLMCall {
		t.Fatalf("expected type %q, got %q", EventLLMCall, evt.Type)
	}
	got, ok := ExtractPayload[LLMCallPayload](evt)
	if !ok {
		t.Fatal("ExtractPayload returned false")
	}
	if got.Phase != "response" {
		t.Fatalf("expected phase %q, got %q", "response", got.Phase)
	}
	if got.TokensInput != 100 {
		t.Fatalf("expected tokens_input 100, got %d", got.TokensInput)
	}
	if got.TokensOutput != 50 {
		t.Fatalf("expected tokens_output 50, got %d", got.TokensOutput)
	}
}

func TestTypedEvent_SkillStarted(t *testing.T) {
	payload := SkillStartedPayload{
		SkillName: "code-review",
		Type:      "workflow",
		Vars:      map[string]string{"repo": "ozzie"},
	}
	evt := NewTypedEvent(SourceSkill, payload)

	if evt.Type != EventSkillStarted {
		t.Fatalf("expected type %q, got %q", EventSkillStarted, evt.Type)
	}
	got, ok := ExtractPayload[SkillStartedPayload](evt)
	if !ok {
		t.Fatal("ExtractPayload returned false")
	}
	if got.SkillName != "code-review" {
		t.Fatalf("expected skill_name %q, got %q", "code-review", got.SkillName)
	}
	if got.Vars["repo"] != "ozzie" {
		t.Fatalf("expected vars[repo]=%q, got %q", "ozzie", got.Vars["repo"])
	}
}

func TestTypedEvent_SkillCompleted(t *testing.T) {
	dur := 3 * time.Second
	payload := SkillCompletedPayload{
		SkillName: "code-review",
		Output:    "LGTM",
		Duration:  dur,
	}
	evt := NewTypedEvent(SourceSkill, payload)

	got, ok := ExtractPayload[SkillCompletedPayload](evt)
	if !ok {
		t.Fatal("ExtractPayload returned false")
	}
	if got.Duration != dur {
		t.Fatalf("expected duration %v, got %v", dur, got.Duration)
	}
	if got.Output != "LGTM" {
		t.Fatalf("expected output %q, got %q", "LGTM", got.Output)
	}
}

func TestTypedEvent_SkillStepStarted(t *testing.T) {
	payload := SkillStepStartedPayload{
		SkillName: "deploy",
		StepID:    "build",
		StepTitle: "Build artifacts",
		Model:     "claude-sonnet",
	}
	evt := NewTypedEvent(SourceSkill, payload)

	if evt.Type != EventSkillStepStarted {
		t.Fatalf("expected type %q, got %q", EventSkillStepStarted, evt.Type)
	}
	got, ok := ExtractPayload[SkillStepStartedPayload](evt)
	if !ok {
		t.Fatal("ExtractPayload returned false")
	}
	if got.StepID != "build" {
		t.Fatalf("expected step_id %q, got %q", "build", got.StepID)
	}
}

func TestTypedEvent_SkillStepCompleted(t *testing.T) {
	dur := 1 * time.Second
	payload := SkillStepCompletedPayload{
		SkillName: "deploy",
		StepID:    "build",
		StepTitle: "Build artifacts",
		Output:    "success",
		Duration:  dur,
	}
	evt := NewTypedEvent(SourceSkill, payload)

	if evt.Type != EventSkillStepCompleted {
		t.Fatalf("expected type %q, got %q", EventSkillStepCompleted, evt.Type)
	}
	got, ok := ExtractPayload[SkillStepCompletedPayload](evt)
	if !ok {
		t.Fatal("ExtractPayload returned false")
	}
	if got.Duration != dur {
		t.Fatalf("expected duration %v, got %v", dur, got.Duration)
	}
}

func TestTypedEventWithSession(t *testing.T) {
	payload := UserMessagePayload{Content: "hello"}
	evt := NewTypedEventWithSession(SourceWS, payload, "sess_abc123")

	if evt.SessionID != "sess_abc123" {
		t.Fatalf("expected session_id %q, got %q", "sess_abc123", evt.SessionID)
	}
	if evt.Source != SourceWS {
		t.Fatalf("expected source %q, got %q", SourceWS, evt.Source)
	}
	got, ok := ExtractPayload[UserMessagePayload](evt)
	if !ok {
		t.Fatal("ExtractPayload returned false")
	}
	if got.Content != "hello" {
		t.Fatalf("expected content %q, got %q", "hello", got.Content)
	}
}

func TestExtractPayload_WrongType(t *testing.T) {
	// Create a UserMessage event, try to extract as ToolCallPayload
	payload := UserMessagePayload{Content: "hello"}
	evt := NewTypedEvent(SourceAgent, payload)

	got, ok := ExtractPayload[ToolCallPayload](evt)
	// Extraction succeeds (JSON round-trip) but fields are zero-valued
	if !ok {
		t.Fatal("ExtractPayload should succeed even for mismatched types (JSON is flexible)")
	}
	if got.Name != "" {
		t.Fatalf("expected empty name for wrong type extraction, got %q", got.Name)
	}
	if got.Status != "" {
		t.Fatalf("expected empty status for wrong type extraction, got %q", got.Status)
	}
}
