package ws

import (
	"encoding/json"
	"testing"
)

func TestMarshalUnmarshal_RequestFrame(t *testing.T) {
	params, _ := json.Marshal(map[string]string{"content": "hello"})
	orig := Frame{
		Type:   FrameTypeRequest,
		ID:     "req-1",
		Method: string(MethodSendMessage),
		Params: params,
	}

	data, err := MarshalFrame(orig)
	if err != nil {
		t.Fatalf("MarshalFrame: %v", err)
	}

	got, err := UnmarshalFrame(data)
	if err != nil {
		t.Fatalf("UnmarshalFrame: %v", err)
	}

	if got.Type != FrameTypeRequest {
		t.Fatalf("expected type %q, got %q", FrameTypeRequest, got.Type)
	}
	if got.ID != "req-1" {
		t.Fatalf("expected id %q, got %q", "req-1", got.ID)
	}
	if got.Method != string(MethodSendMessage) {
		t.Fatalf("expected method %q, got %q", MethodSendMessage, got.Method)
	}

	var p map[string]string
	if err := json.Unmarshal(got.Params, &p); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if p["content"] != "hello" {
		t.Fatalf("expected params.content %q, got %q", "hello", p["content"])
	}
}

func TestMarshalUnmarshal_ResponseFrame(t *testing.T) {
	ok := true
	payload, _ := json.Marshal(map[string]string{"session_id": "sess_123"})
	orig := Frame{
		Type:    FrameTypeResponse,
		ID:      "req-1",
		OK:      &ok,
		Payload: payload,
	}

	data, err := MarshalFrame(orig)
	if err != nil {
		t.Fatalf("MarshalFrame: %v", err)
	}

	got, err := UnmarshalFrame(data)
	if err != nil {
		t.Fatalf("UnmarshalFrame: %v", err)
	}

	if got.Type != FrameTypeResponse {
		t.Fatalf("expected type %q, got %q", FrameTypeResponse, got.Type)
	}
	if got.OK == nil || !*got.OK {
		t.Fatal("expected ok=true")
	}
}

func TestMarshalUnmarshal_EventFrame(t *testing.T) {
	payload, _ := json.Marshal(map[string]string{"content": "world"})
	orig := Frame{
		Type:      FrameTypeEvent,
		Event:     "assistant.stream",
		SessionID: "sess_abc",
		Payload:   payload,
	}

	data, err := MarshalFrame(orig)
	if err != nil {
		t.Fatalf("MarshalFrame: %v", err)
	}

	got, err := UnmarshalFrame(data)
	if err != nil {
		t.Fatalf("UnmarshalFrame: %v", err)
	}

	if got.Type != FrameTypeEvent {
		t.Fatalf("expected type %q, got %q", FrameTypeEvent, got.Type)
	}
	if got.Event != "assistant.stream" {
		t.Fatalf("expected event %q, got %q", "assistant.stream", got.Event)
	}
	if got.SessionID != "sess_abc" {
		t.Fatalf("expected session_id %q, got %q", "sess_abc", got.SessionID)
	}
}

func TestNewEventFrame(t *testing.T) {
	f, err := NewEventFrame("user.message", "sess_42", map[string]string{"content": "hi"})
	if err != nil {
		t.Fatalf("NewEventFrame: %v", err)
	}
	if f.Type != FrameTypeEvent {
		t.Fatalf("expected type %q, got %q", FrameTypeEvent, f.Type)
	}
	if f.Event != "user.message" {
		t.Fatalf("expected event %q, got %q", "user.message", f.Event)
	}
	if f.SessionID != "sess_42" {
		t.Fatalf("expected session_id %q, got %q", "sess_42", f.SessionID)
	}

	var p map[string]string
	if err := json.Unmarshal(f.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p["content"] != "hi" {
		t.Fatalf("expected payload.content %q, got %q", "hi", p["content"])
	}
}

func TestNewResponseFrame_OK(t *testing.T) {
	f, err := NewResponseFrame("req-5", true, map[string]string{"status": "done"}, "")
	if err != nil {
		t.Fatalf("NewResponseFrame: %v", err)
	}
	if f.Type != FrameTypeResponse {
		t.Fatalf("expected type %q, got %q", FrameTypeResponse, f.Type)
	}
	if f.ID != "req-5" {
		t.Fatalf("expected id %q, got %q", "req-5", f.ID)
	}
	if f.OK == nil || !*f.OK {
		t.Fatal("expected ok=true")
	}
	if f.Error != "" {
		t.Fatalf("expected no error, got %q", f.Error)
	}

	var p map[string]string
	if err := json.Unmarshal(f.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p["status"] != "done" {
		t.Fatalf("expected payload.status %q, got %q", "done", p["status"])
	}
}

func TestNewResponseFrame_Error(t *testing.T) {
	f, err := NewResponseFrame("req-6", false, nil, "something went wrong")
	if err != nil {
		t.Fatalf("NewResponseFrame: %v", err)
	}
	if f.OK == nil || *f.OK {
		t.Fatal("expected ok=false")
	}
	if f.Error != "something went wrong" {
		t.Fatalf("expected error %q, got %q", "something went wrong", f.Error)
	}
	if f.Payload != nil {
		t.Fatalf("expected nil payload, got %s", string(f.Payload))
	}
}
