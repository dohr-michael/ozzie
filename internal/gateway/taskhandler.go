package gateway

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/tasks"
)

// WSTaskHandler implements ws.TaskHandler using the task submitter.
type WSTaskHandler struct {
	pool tasks.TaskSubmitter
}

// NewWSTaskHandler creates a new WS task handler.
func NewWSTaskHandler(pool tasks.TaskSubmitter) *WSTaskHandler {
	return &WSTaskHandler{pool: pool}
}

type taskSummary struct {
	ID       string             `json:"id"`
	Title    string             `json:"title"`
	Status   tasks.TaskStatus   `json:"status"`
	Progress tasks.TaskProgress `json:"progress"`
}

// Submit creates a new task via the pool.
func (h *WSTaskHandler) Submit(sessionID, title, description string, tools []string, priority string) (string, error) {
	p := tasks.PriorityNormal
	if priority != "" {
		p = tasks.TaskPriority(priority)
	}

	// Default tools if none specified
	if len(tools) == 0 {
		tools = []string{"run_command", "git", "query_memories"}
	}

	t := &tasks.Task{
		SessionID:   sessionID,
		Title:       title,
		Description: description,
		Priority:    p,
		Config: tasks.TaskConfig{
			Tools: tools,
		},
	}

	if err := h.pool.Submit(t); err != nil {
		return "", err
	}
	return t.ID, nil
}

// Check returns the status of a task.
func (h *WSTaskHandler) Check(taskID string) (any, error) {
	t, err := h.pool.Store().Get(taskID)
	if err != nil {
		return nil, err
	}

	result := taskSummary{
		ID:       t.ID,
		Title:    t.Title,
		Status:   t.Status,
		Progress: t.Progress,
	}

	return result, nil
}

// Cancel cancels a task.
func (h *WSTaskHandler) Cancel(taskID string, reason string) error {
	if reason == "" {
		reason = "cancelled via WS"
	}
	return h.pool.Cancel(taskID, reason)
}

// ReplyTask sends user feedback to a suspended coordinator task and resumes it.
func (h *WSTaskHandler) ReplyTask(taskID string, feedback string, status string, sessionID string) error {
	store := h.pool.Store()

	task, err := store.Get(taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}
	if task.Status != tasks.TaskSuspended {
		return fmt.Errorf("task %s is not suspended (status: %s)", taskID, task.Status)
	}
	if !task.WaitingForReply {
		return fmt.Errorf("task %s is not waiting for a reply", taskID)
	}

	if status != "approved" && status != "revise" {
		return fmt.Errorf("invalid status %q: must be 'approved' or 'revise'", status)
	}

	// Find pending request token
	mailbox, err := store.LoadMailbox(taskID)
	if err != nil {
		return fmt.Errorf("load mailbox: %w", err)
	}

	token := findPendingToken(mailbox)
	if token == "" {
		return fmt.Errorf("no pending request found in task mailbox")
	}

	// Append response
	msg := tasks.MailboxMessage{
		ID:        uuid.New().String(),
		Ts:        time.Now(),
		Type:      "response",
		Token:     token,
		Content:   feedback,
		Status:    status,
		SessionID: sessionID,
	}
	if err := store.AppendMailbox(taskID, msg); err != nil {
		return fmt.Errorf("append mailbox: %w", err)
	}

	return h.pool.ResumeTask(taskID)
}

// findPendingToken returns the token of the last unanswered request in a mailbox.
func findPendingToken(mailbox []tasks.MailboxMessage) string {
	responded := make(map[string]bool)
	for _, msg := range mailbox {
		if msg.Type == "response" {
			responded[msg.Token] = true
		}
	}
	for i := len(mailbox) - 1; i >= 0; i-- {
		if mailbox[i].Type == "request" && !responded[mailbox[i].Token] {
			return mailbox[i].Token
		}
	}
	return ""
}

// List returns all tasks for a session.
func (h *WSTaskHandler) List(sessionID string) (any, error) {
	filter := tasks.ListFilter{}
	if sessionID != "" {
		filter.SessionID = sessionID
	}

	list, err := h.pool.Store().List(filter)
	if err != nil {
		return nil, err
	}

	summaries := make([]taskSummary, len(list))
	for i, t := range list {
		summaries[i] = taskSummary{
			ID:       t.ID,
			Title:    t.Title,
			Status:   t.Status,
			Progress: t.Progress,
		}
	}

	return summaries, nil
}
