package tui

import (
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dohr-michael/ozzie/internal/events"
	ws "github.com/dohr-michael/ozzie/internal/gateway/ws"
)

// Project converts a gateway WS Frame into a typed tea.Msg.
// Returns nil for frames that don't map to a TUI message.
func Project(frame ws.Frame) tea.Msg {
	if frame.Event == "" {
		return nil
	}

	switch events.EventType(frame.Event) {
	case events.EventAssistantStream:
		return projectStream(frame)
	case events.EventAssistantMessage:
		return projectAssistantMessage(frame)
	case events.EventToolCall:
		return projectToolCall(frame)
	case events.EventPromptRequest:
		return projectPromptRequest(frame)
	case events.EventLLMCall:
		return projectLLMCall(frame)
	case events.EventSkillStarted:
		return projectSkillStarted(frame)
	case events.EventSkillCompleted:
		return projectSkillCompleted(frame)
	case events.EventSkillStepStarted:
		return projectSkillStepStarted(frame)
	case events.EventSkillStepCompleted:
		return projectSkillStepCompleted(frame)
	default:
		return nil
	}
}

func projectStream(frame ws.Frame) tea.Msg {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return nil
	}
	payload, ok := events.GetAssistantStreamPayload(evt)
	if !ok {
		return nil
	}
	switch payload.Phase {
	case events.StreamPhaseStart:
		return StreamStartMsg{}
	case events.StreamPhaseDelta:
		return StreamDeltaMsg{Content: payload.Content, Index: payload.Index}
	case events.StreamPhaseEnd:
		return StreamEndMsg{}
	default:
		return nil
	}
}

func projectAssistantMessage(frame ws.Frame) tea.Msg {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return nil
	}
	payload, ok := events.GetAssistantMessagePayload(evt)
	if !ok {
		return nil
	}
	return AssistantMessageMsg{Content: payload.Content, Error: payload.Error}
}

func projectToolCall(frame ws.Frame) tea.Msg {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return nil
	}
	payload, ok := events.GetToolCallPayload(evt)
	if !ok {
		return nil
	}
	return ToolCallMsg{
		Status:    string(payload.Status),
		Name:      payload.Name,
		Arguments: payload.Arguments,
		Result:    payload.Result,
		Error:     payload.Error,
	}
}

func projectPromptRequest(frame ws.Frame) tea.Msg {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return nil
	}
	payload, ok := events.GetPromptRequestPayload(evt)
	if !ok {
		return nil
	}
	return PromptRequestMsg{
		Type:        string(payload.Type),
		Label:       payload.Label,
		Options:     payload.Options,
		Token:       payload.Token,
		HelpText:    payload.HelpText,
		Placeholder: payload.Placeholder,
		MinSelect:   payload.MinSelect,
		MaxSelect:   payload.MaxSelect,
	}
}

func projectLLMCall(frame ws.Frame) tea.Msg {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return nil
	}
	payload, ok := events.GetLLMCallPayload(evt)
	if !ok {
		return nil
	}
	if payload.Phase != "completed" {
		return nil
	}
	return LLMTelemetryMsg{
		Model:     payload.Model,
		TokensIn:  payload.TokensInput,
		TokensOut: payload.TokensOutput,
	}
}

func projectSkillStarted(frame ws.Frame) tea.Msg {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return nil
	}
	payload, ok := events.GetSkillStartedPayload(evt)
	if !ok {
		return nil
	}
	return SkillStartedMsg{Name: payload.SkillName}
}

func projectSkillCompleted(frame ws.Frame) tea.Msg {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return nil
	}
	payload, ok := events.GetSkillCompletedPayload(evt)
	if !ok {
		return nil
	}
	return SkillCompletedMsg{
		Name:     payload.SkillName,
		Error:    payload.Error,
		Duration: payload.Duration,
	}
}

func projectSkillStepStarted(frame ws.Frame) tea.Msg {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return nil
	}
	payload, ok := events.GetSkillStepStartedPayload(evt)
	if !ok {
		return nil
	}
	return SkillStepStartedMsg{
		SkillName: payload.SkillName,
		StepID:    payload.StepID,
		StepTitle: payload.StepTitle,
	}
}

func projectSkillStepCompleted(frame ws.Frame) tea.Msg {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return nil
	}
	payload, ok := events.GetSkillStepCompletedPayload(evt)
	if !ok {
		return nil
	}
	return SkillStepCompletedMsg{
		SkillName: payload.SkillName,
		StepID:    payload.StepID,
		Error:     payload.Error,
		Duration:  payload.Duration,
	}
}
