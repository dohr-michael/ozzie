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

// =============================================================================
// SKILL EVENTS
// =============================================================================

type SkillStartedPayload struct {
	SkillName string            `json:"skill_name"`
	Type      string            `json:"type"`
	Vars      map[string]string `json:"vars,omitempty"`
}

func (SkillStartedPayload) EventType() EventType { return EventSkillStarted }

type SkillCompletedPayload struct {
	SkillName string        `json:"skill_name"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
}

func (SkillCompletedPayload) EventType() EventType { return EventSkillCompleted }

type SkillStepStartedPayload struct {
	SkillName string `json:"skill_name"`
	StepID    string `json:"step_id"`
	StepTitle string `json:"step_title"`
	Model     string `json:"model"`
}

func (SkillStepStartedPayload) EventType() EventType { return EventSkillStepStarted }

type SkillStepCompletedPayload struct {
	SkillName string        `json:"skill_name"`
	StepID    string        `json:"step_id"`
	StepTitle string        `json:"step_title"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
}

func (SkillStepCompletedPayload) EventType() EventType { return EventSkillStepCompleted }

func GetSkillStartedPayload(e Event) (SkillStartedPayload, bool) {
	return ExtractPayload[SkillStartedPayload](e)
}

func GetSkillCompletedPayload(e Event) (SkillCompletedPayload, bool) {
	return ExtractPayload[SkillCompletedPayload](e)
}

func GetSkillStepStartedPayload(e Event) (SkillStepStartedPayload, bool) {
	return ExtractPayload[SkillStepStartedPayload](e)
}

func GetSkillStepCompletedPayload(e Event) (SkillStepCompletedPayload, bool) {
	return ExtractPayload[SkillStepCompletedPayload](e)
}

// =============================================================================
// TASK EVENTS
// =============================================================================

type TaskCreatedPayload struct {
	TaskID      string `json:"task_id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
}

func (TaskCreatedPayload) EventType() EventType { return EventTaskCreated }

type TaskStartedPayload struct {
	TaskID string `json:"task_id"`
	Title  string `json:"title"`
}

func (TaskStartedPayload) EventType() EventType { return EventTaskStarted }

type TaskProgressPayload struct {
	TaskID           string `json:"task_id"`
	CurrentStep      int    `json:"current_step"`
	TotalSteps       int    `json:"total_steps"`
	CurrentStepLabel string `json:"current_step_label,omitempty"`
	Percentage       int    `json:"percentage"`
}

func (TaskProgressPayload) EventType() EventType { return EventTaskProgress }

type TaskCompletedPayload struct {
	TaskID        string        `json:"task_id"`
	Title         string        `json:"title"`
	OutputSummary string        `json:"output_summary,omitempty"`
	Duration      time.Duration `json:"duration,omitempty"`
	TokensInput   int           `json:"tokens_input,omitempty"`
	TokensOutput  int           `json:"tokens_output,omitempty"`
}

func (TaskCompletedPayload) EventType() EventType { return EventTaskCompleted }

type TaskFailedPayload struct {
	TaskID     string `json:"task_id"`
	Title      string `json:"title"`
	Error      string `json:"error"`
	RetryCount int    `json:"retry_count"`
	WillRetry  bool   `json:"will_retry"`
}

func (TaskFailedPayload) EventType() EventType { return EventTaskFailed }

type TaskCancelledPayload struct {
	TaskID string `json:"task_id"`
	Reason string `json:"reason,omitempty"`
}

func (TaskCancelledPayload) EventType() EventType { return EventTaskCancelled }

func GetTaskCreatedPayload(e Event) (TaskCreatedPayload, bool) {
	return ExtractPayload[TaskCreatedPayload](e)
}

func GetTaskStartedPayload(e Event) (TaskStartedPayload, bool) {
	return ExtractPayload[TaskStartedPayload](e)
}

func GetTaskProgressPayload(e Event) (TaskProgressPayload, bool) {
	return ExtractPayload[TaskProgressPayload](e)
}

func GetTaskCompletedPayload(e Event) (TaskCompletedPayload, bool) {
	return ExtractPayload[TaskCompletedPayload](e)
}

func GetTaskFailedPayload(e Event) (TaskFailedPayload, bool) {
	return ExtractPayload[TaskFailedPayload](e)
}

func GetTaskCancelledPayload(e Event) (TaskCancelledPayload, bool) {
	return ExtractPayload[TaskCancelledPayload](e)
}

type TaskSuspendedPayload struct {
	TaskID       string `json:"task_id"`
	Title        string `json:"title"`
	Reason       string `json:"reason,omitempty"`
	SuspendCount int    `json:"suspend_count"`
	PlanContent  string `json:"plan_content,omitempty"`
	Token        string `json:"token,omitempty"`
}

func (TaskSuspendedPayload) EventType() EventType { return EventTaskSuspended }

type TaskResumedPayload struct {
	TaskID string `json:"task_id"`
	Title  string `json:"title"`
}

func (TaskResumedPayload) EventType() EventType { return EventTaskResumed }

func GetTaskSuspendedPayload(e Event) (TaskSuspendedPayload, bool) {
	return ExtractPayload[TaskSuspendedPayload](e)
}

func GetTaskResumedPayload(e Event) (TaskResumedPayload, bool) {
	return ExtractPayload[TaskResumedPayload](e)
}

// =============================================================================
// SCHEDULER EVENTS
// =============================================================================

type ScheduleTriggerPayload struct {
	EntryID   string `json:"entry_id,omitempty"`
	SkillName string `json:"skill_name"`
	Trigger   string `json:"trigger"`
	TaskID    string `json:"task_id"`
}

func (ScheduleTriggerPayload) EventType() EventType { return EventScheduleTrigger }

func GetScheduleTriggerPayload(e Event) (ScheduleTriggerPayload, bool) {
	return ExtractPayload[ScheduleTriggerPayload](e)
}

type ScheduleCreatedPayload struct {
	EntryID     string `json:"entry_id"`
	Title       string `json:"title"`
	Source      string `json:"source"`
	CronSpec    string `json:"cron_spec,omitempty"`
	IntervalSec int    `json:"interval_sec,omitempty"`
}

func (ScheduleCreatedPayload) EventType() EventType { return EventScheduleCreated }

func GetScheduleCreatedPayload(e Event) (ScheduleCreatedPayload, bool) {
	return ExtractPayload[ScheduleCreatedPayload](e)
}

type ScheduleRemovedPayload struct {
	EntryID string `json:"entry_id"`
	Title   string `json:"title"`
}

func (ScheduleRemovedPayload) EventType() EventType { return EventScheduleRemoved }

func GetScheduleRemovedPayload(e Event) (ScheduleRemovedPayload, bool) {
	return ExtractPayload[ScheduleRemovedPayload](e)
}

// =============================================================================
// VERIFICATION EVENTS
// =============================================================================

type TaskVerificationPayload struct {
	TaskID    string   `json:"task_id"`
	SkillName string   `json:"skill_name"`
	StepID    string   `json:"step_id"`
	Pass      bool     `json:"pass"`
	Score     int      `json:"score"`
	Issues    []string `json:"issues,omitempty"`
	Attempt   int      `json:"attempt"`
}

func (TaskVerificationPayload) EventType() EventType { return EventTaskVerification }

func GetTaskVerificationPayload(e Event) (TaskVerificationPayload, bool) {
	return ExtractPayload[TaskVerificationPayload](e)
}
