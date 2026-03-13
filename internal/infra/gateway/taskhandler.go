package gateway

import (
	"github.com/dohr-michael/ozzie/internal/infra/tasks"
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
		tools = []string{"run_command", "git"}
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

// QueryTasks returns a single task detail (when taskID is set) or a list of task summaries.
func (h *WSTaskHandler) QueryTasks(taskID, sessionID string) (any, error) {
	if taskID != "" {
		t, err := h.pool.Store().Get(taskID)
		if err != nil {
			return nil, err
		}
		return taskSummary{
			ID:       t.ID,
			Title:    t.Title,
			Status:   t.Status,
			Progress: t.Progress,
		}, nil
	}

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

// Cancel cancels a task.
func (h *WSTaskHandler) Cancel(taskID string, reason string) error {
	if reason == "" {
		reason = "cancelled via WS"
	}
	return h.pool.Cancel(taskID, reason)
}
