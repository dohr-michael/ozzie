package plugins

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/tasks"
)

// ListTasksTool lists tasks with optional filters.
type ListTasksTool struct {
	store tasks.Store
}

// NewListTasksTool creates a new list_tasks tool.
func NewListTasksTool(store tasks.Store) *ListTasksTool {
	return &ListTasksTool{store: store}
}

// ListTasksManifest returns the plugin manifest for the list_tasks tool.
func ListTasksManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "list_tasks",
		Description: "List all tasks with optional status filter",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "list_tasks",
				Description: "List all tasks with optional status filter. Returns task IDs, titles, statuses, and progress.",
				Parameters: map[string]ParamSpec{
					"status": {
						Type:        "string",
						Description: "Filter by status: pending, running, completed, failed, cancelled",
						Enum:        []string{"pending", "running", "completed", "failed", "cancelled"},
					},
					"session_id": {
						Type:        "string",
						Description: "Filter by session ID",
					},
				},
			},
		},
	}
}

type listTasksInput struct {
	Status    string `json:"status"`
	SessionID string `json:"session_id"`
}

type listTasksEntry struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Status    tasks.TaskStatus   `json:"status"`
	Progress  tasks.TaskProgress `json:"progress"`
	DependsOn []string           `json:"depends_on,omitempty"`
	CreatedAt string             `json:"created_at"`
}

// Info returns the tool info for Eino registration.
func (t *ListTasksTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ListTasksManifest().Tools[0]), nil
}

// InvokableRun lists tasks with optional filters.
func (t *ListTasksTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input listTasksInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("list_tasks: parse input: %w", err)
	}

	filter := tasks.ListFilter{
		Status:    tasks.TaskStatus(input.Status),
		SessionID: input.SessionID,
	}

	all, err := t.store.List(filter)
	if err != nil {
		return "", fmt.Errorf("list_tasks: %w", err)
	}

	// Cap at 20 most recent
	if len(all) > 20 {
		all = all[:20]
	}

	entries := make([]listTasksEntry, len(all))
	for i, task := range all {
		entries[i] = listTasksEntry{
			ID:        task.ID,
			Title:     task.Title,
			Status:    task.Status,
			Progress:  task.Progress,
			DependsOn: task.DependsOn,
			CreatedAt: task.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	result, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("list_tasks: marshal: %w", err)
	}
	return string(result), nil
}

var _ tool.InvokableTool = (*ListTasksTool)(nil)
