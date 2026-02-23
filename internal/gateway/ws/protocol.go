package ws

import "encoding/json"

// FrameType represents the type of WebSocket frame.
type FrameType string

const (
	FrameTypeRequest  FrameType = "req"
	FrameTypeResponse FrameType = "res"
	FrameTypeEvent    FrameType = "event"
)

// Method represents a WebSocket request method.
type Method string

const (
	MethodSendMessage    Method = "send_message"
	MethodOpenSession    Method = "open_session"
	MethodPromptResponse Method = "prompt_response"
	MethodSubmitTask     Method = "submit_task"
	MethodCheckTask      Method = "check_task"
	MethodCancelTask     Method = "cancel_task"
	MethodListTasks      Method = "list_tasks"
	MethodAcceptAllTools Method = "accept_all_tools"
	MethodReplyTask     Method = "reply_task"
)

// Frame is the WebSocket protocol envelope.
type Frame struct {
	Type      FrameType       `json:"type"`
	ID        string          `json:"id,omitempty"`
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	OK        *bool           `json:"ok,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     string          `json:"error,omitempty"`
	Event     string          `json:"event,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
}

// MarshalFrame serializes a Frame to JSON bytes.
func MarshalFrame(f Frame) ([]byte, error) {
	return json.Marshal(f)
}

// UnmarshalFrame deserializes JSON bytes into a Frame.
func UnmarshalFrame(data []byte) (Frame, error) {
	var f Frame
	err := json.Unmarshal(data, &f)
	return f, err
}

// NewEventFrame creates a Frame for broadcasting an event.
func NewEventFrame(event string, sessionID string, payload any) (Frame, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Frame{}, err
	}
	return Frame{
		Type:      FrameTypeEvent,
		Event:     event,
		SessionID: sessionID,
		Payload:   data,
	}, nil
}

// NewResponseFrame creates a response Frame.
func NewResponseFrame(id string, ok bool, payload any, errMsg string) (Frame, error) {
	f := Frame{
		Type:  FrameTypeResponse,
		ID:    id,
		OK:    &ok,
		Error: errMsg,
	}
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return Frame{}, err
		}
		f.Payload = data
	}
	return f, nil
}
