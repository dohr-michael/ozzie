package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/tasks"
)

// =============================================================================
// request_validation
// =============================================================================

// RequestValidationTool sends a validation request to the user and self-suspends.
type RequestValidationTool struct{}

// NewRequestValidationTool creates a new request_validation tool.
func NewRequestValidationTool() *RequestValidationTool {
	return &RequestValidationTool{}
}

// RequestValidationManifest returns the plugin manifest for request_validation.
func RequestValidationManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "request_validation",
		Description: "Request user validation for a plan before execution",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "request_validation",
				Description: "Submit a plan or proposal for user review. The task will be suspended until the user responds. Use this BEFORE executing any implementation — present your plan and wait for approval.",
				Parameters: map[string]ParamSpec{
					"plan": {
						Type:        "string",
						Description: "The detailed plan or proposal to present to the user for validation",
						Required:    true,
					},
				},
			},
		},
	}
}

type requestValidationInput struct {
	Plan string `json:"plan"`
}

// Info returns the tool info for Eino registration.
func (t *RequestValidationTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&RequestValidationManifest().Tools[0]), nil
}

// InvokableRun sends a validation request through the side channel.
func (t *RequestValidationTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input requestValidationInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("request_validation: parse input: %w", err)
	}
	if input.Plan == "" {
		return "", fmt.Errorf("request_validation: plan is required")
	}

	ch := events.ValidationChFromContext(ctx)
	if ch == nil {
		return "", fmt.Errorf("request_validation: not available outside coordinator tasks")
	}

	token := uuid.New().String()

	// Send the signal — buffered channel (size 1), won't block
	select {
	case ch <- events.ValidationRequest{Token: token, Content: input.Plan}:
	default:
		return "", fmt.Errorf("request_validation: validation already pending")
	}

	result, _ := json.Marshal(map[string]string{
		"status": "suspended",
		"token":  token,
		"note":   "Task suspended. Will resume when user replies.",
	})
	return string(result), nil
}

var _ tool.InvokableTool = (*RequestValidationTool)(nil)

// =============================================================================
// reply_task
// =============================================================================

// ReplyTaskTool sends user feedback to a suspended coordinator task.
type ReplyTaskTool struct {
	pool tasks.TaskSubmitter
}

// NewReplyTaskTool creates a new reply_task tool.
func NewReplyTaskTool(pool tasks.TaskSubmitter) *ReplyTaskTool {
	return &ReplyTaskTool{pool: pool}
}

// ReplyTaskManifest returns the plugin manifest for reply_task.
func ReplyTaskManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "reply_task",
		Description: "Reply to a suspended task waiting for user validation",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "reply_task",
				Description: "Send feedback to a suspended coordinator task that requested validation. The task will resume execution with the provided feedback.",
				Parameters: map[string]ParamSpec{
					"task_id": {
						Type:        "string",
						Description: "The ID of the suspended task to reply to",
						Required:    true,
					},
					"feedback": {
						Type:        "string",
						Description: "Your feedback or approval for the task's plan. E.g. 'Approved, go ahead' or 'Change X to Y'",
						Required:    true,
					},
					"status": {
						Type:        "string",
						Description: "Whether the plan is approved or needs revision",
						Required:    true,
						Enum:        []string{"approved", "revise"},
					},
				},
			},
		},
	}
}

type replyTaskInput struct {
	TaskID   string `json:"task_id"`
	Feedback string `json:"feedback"`
	Status   string `json:"status"` // "approved" | "revise"
}

// Info returns the tool info for Eino registration.
func (t *ReplyTaskTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ReplyTaskManifest().Tools[0]), nil
}

// InvokableRun appends user feedback to the task's mailbox and resumes it.
func (t *ReplyTaskTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input replyTaskInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("reply_task: parse input: %w", err)
	}
	if input.TaskID == "" {
		return "", fmt.Errorf("reply_task: task_id is required")
	}
	if input.Feedback == "" {
		return "", fmt.Errorf("reply_task: feedback is required")
	}
	if input.Status != "approved" && input.Status != "revise" {
		return "", fmt.Errorf("reply_task: status must be 'approved' or 'revise'")
	}

	store := t.pool.Store()

	// Verify task is suspended and waiting
	task, err := store.Get(input.TaskID)
	if err != nil {
		return "", fmt.Errorf("reply_task: %w", err)
	}
	if task.Status != tasks.TaskSuspended {
		return "", fmt.Errorf("reply_task: task %s is not suspended (status: %s)", input.TaskID, task.Status)
	}
	if !task.WaitingForReply {
		return "", fmt.Errorf("reply_task: task %s is not waiting for a reply", input.TaskID)
	}

	// Find the pending request token from mailbox
	mailbox, err := store.LoadMailbox(input.TaskID)
	if err != nil {
		return "", fmt.Errorf("reply_task: load mailbox: %w", err)
	}

	token := findPendingRequestToken(mailbox)
	if token == "" {
		return "", fmt.Errorf("reply_task: no pending request found in mailbox")
	}

	sessionID := events.SessionIDFromContext(ctx)

	// Append response to mailbox
	msg := tasks.MailboxMessage{
		ID:        uuid.New().String(),
		Ts:        time.Now(),
		Type:      "response",
		Token:     token,
		Content:   input.Feedback,
		Status:    input.Status,
		SessionID: sessionID,
	}
	if err := store.AppendMailbox(input.TaskID, msg); err != nil {
		return "", fmt.Errorf("reply_task: append mailbox: %w", err)
	}

	// Resume the task
	if err := t.pool.ResumeTask(input.TaskID); err != nil {
		return "", fmt.Errorf("reply_task: resume: %w", err)
	}

	result, _ := json.Marshal(map[string]string{
		"task_id": input.TaskID,
		"status":  "resumed",
	})
	return string(result), nil
}

// findPendingRequestToken returns the token of the last request without a matching response.
func findPendingRequestToken(mailbox []tasks.MailboxMessage) string {
	responded := make(map[string]bool)
	for _, msg := range mailbox {
		if msg.Type == "response" {
			responded[msg.Token] = true
		}
	}

	// Walk backwards to find the latest unanswered request
	for i := len(mailbox) - 1; i >= 0; i-- {
		if mailbox[i].Type == "request" && !responded[mailbox[i].Token] {
			return mailbox[i].Token
		}
	}
	return ""
}

var _ tool.InvokableTool = (*ReplyTaskTool)(nil)
