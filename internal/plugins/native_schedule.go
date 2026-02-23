package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/scheduler"
)

// =============================================================================
// schedule_task
// =============================================================================

// ScheduleTaskTool creates a recurring schedule entry.
type ScheduleTaskTool struct {
	sched *scheduler.Scheduler
	bus   *events.Bus
}

// NewScheduleTaskTool creates a new schedule_task tool.
func NewScheduleTaskTool(sched *scheduler.Scheduler, bus *events.Bus) *ScheduleTaskTool {
	return &ScheduleTaskTool{sched: sched, bus: bus}
}

// ScheduleTaskManifest returns the plugin manifest for the schedule_task tool.
func ScheduleTaskManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "schedule_task",
		Description: "Create a recurring scheduled task",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "schedule_task",
				Description: "Create a recurring scheduled task that runs on a cron schedule, at a fixed interval, or in response to an event. Returns the schedule entry ID.",
				Parameters: map[string]ParamSpec{
					"title": {
						Type:        "string",
						Description: "Short title for the schedule",
						Required:    true,
					},
					"description": {
						Type:        "string",
						Description: "Detailed description of what the recurring task should do",
						Required:    true,
					},
					"cron": {
						Type:        "string",
						Description: "5-field cron expression (e.g. \"*/5 * * * *\" for every 5 minutes). Mutually exclusive with interval and on_event.",
					},
					"interval": {
						Type:        "string",
						Description: "Go duration string for fixed intervals (e.g. \"30s\", \"5m\", \"1h\"). Minimum 5s. Mutually exclusive with cron and on_event.",
					},
					"on_event": {
						Type:        "string",
						Description: "Event type to trigger on (e.g. \"task.completed\"). Mutually exclusive with cron and interval.",
					},
					"tools": {
						Type:        "array",
						Description: "Tool names the scheduled task agent can use",
						Items: &ParamSpec{
							Type: "string",
						},
					},
					"work_dir": {
						Type:        "string",
						Description: "Working directory for the task",
					},
					"env": {
						Type:        "object",
						Description: "Environment variables for the task",
					},
					"cooldown": {
						Type:        "string",
						Description: "Minimum time between triggers (Go duration, default \"60s\")",
					},
					"max_runs": {
						Type:        "integer",
						Description: "Maximum number of runs before auto-disabling (0 = unlimited)",
					},
				},
			},
		},
	}
}

type scheduleTaskInput struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Cron        string            `json:"cron"`
	Interval    string            `json:"interval"`
	OnEvent     string            `json:"on_event"`
	Tools       []string          `json:"tools"`
	WorkDir     string            `json:"work_dir"`
	Env         map[string]string `json:"env"`
	Cooldown    string            `json:"cooldown"`
	MaxRuns     int               `json:"max_runs"`
}

// Info returns the tool info for Eino registration.
func (t *ScheduleTaskTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ScheduleTaskManifest().Tools[0]), nil
}

// InvokableRun creates a schedule entry and returns its ID.
func (t *ScheduleTaskTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input scheduleTaskInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("schedule_task: parse input: %w", err)
	}
	if input.Title == "" {
		return "", fmt.Errorf("schedule_task: title is required")
	}
	if input.Description == "" {
		return "", fmt.Errorf("schedule_task: description is required")
	}

	// Exactly one trigger type required
	triggerCount := 0
	if input.Cron != "" {
		triggerCount++
	}
	if input.Interval != "" {
		triggerCount++
	}
	if input.OnEvent != "" {
		triggerCount++
	}
	if triggerCount == 0 {
		return "", fmt.Errorf("schedule_task: one of cron, interval, or on_event is required")
	}
	if triggerCount > 1 {
		return "", fmt.Errorf("schedule_task: cron, interval, and on_event are mutually exclusive")
	}

	entry := &scheduler.ScheduleEntry{
		Source:      "dynamic",
		SessionID:   events.SessionIDFromContext(ctx),
		Title:       input.Title,
		Description: input.Description,
		MaxRuns:     input.MaxRuns,
		Enabled:     true,
		TaskTemplate: &scheduler.TaskTemplate{
			Title:       input.Title,
			Description: input.Description,
			Tools:       input.Tools,
			WorkDir:     input.WorkDir,
			Env:         input.Env,
		},
	}

	// Parse trigger
	if input.Cron != "" {
		entry.CronSpec = input.Cron
	}
	if input.Interval != "" {
		d, err := time.ParseDuration(input.Interval)
		if err != nil {
			return "", fmt.Errorf("schedule_task: invalid interval %q: %w", input.Interval, err)
		}
		entry.IntervalSec = int(d.Seconds())
	}
	if input.OnEvent != "" {
		entry.OnEvent = &scheduler.EventTrigger{Event: input.OnEvent}
	}

	// Parse cooldown
	if input.Cooldown != "" {
		d, err := time.ParseDuration(input.Cooldown)
		if err != nil {
			return "", fmt.Errorf("schedule_task: invalid cooldown %q: %w", input.Cooldown, err)
		}
		entry.CooldownSec = int(d.Seconds())
	}

	if err := t.sched.AddEntry(entry); err != nil {
		return "", fmt.Errorf("schedule_task: %w", err)
	}

	// Emit created event
	t.bus.Publish(events.NewTypedEvent(events.SourceScheduler, events.ScheduleCreatedPayload{
		EntryID:     entry.ID,
		Title:       entry.Title,
		Source:      entry.Source,
		CronSpec:    entry.CronSpec,
		IntervalSec: entry.IntervalSec,
	}))

	result, _ := json.Marshal(map[string]any{
		"entry_id": entry.ID,
		"status":   "created",
		"title":    entry.Title,
	})
	return string(result), nil
}

var _ tool.InvokableTool = (*ScheduleTaskTool)(nil)

// =============================================================================
// unschedule_task
// =============================================================================

// UnscheduleTaskTool removes a dynamic schedule entry.
type UnscheduleTaskTool struct {
	sched *scheduler.Scheduler
	bus   *events.Bus
}

// NewUnscheduleTaskTool creates a new unschedule_task tool.
func NewUnscheduleTaskTool(sched *scheduler.Scheduler, bus *events.Bus) *UnscheduleTaskTool {
	return &UnscheduleTaskTool{sched: sched, bus: bus}
}

// UnscheduleTaskManifest returns the plugin manifest for the unschedule_task tool.
func UnscheduleTaskManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "unschedule_task",
		Description: "Remove a scheduled task",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "unschedule_task",
				Description: "Remove a dynamic schedule entry by its ID. Skill-based schedules cannot be removed.",
				Parameters: map[string]ParamSpec{
					"entry_id": {
						Type:        "string",
						Description: "The schedule entry ID to remove (sched_... prefix)",
						Required:    true,
					},
				},
			},
		},
	}
}

type unscheduleTaskInput struct {
	EntryID string `json:"entry_id"`
}

// Info returns the tool info for Eino registration.
func (t *UnscheduleTaskTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&UnscheduleTaskManifest().Tools[0]), nil
}

// InvokableRun removes a schedule entry.
func (t *UnscheduleTaskTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input unscheduleTaskInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("unschedule_task: parse input: %w", err)
	}
	if input.EntryID == "" {
		return "", fmt.Errorf("unschedule_task: entry_id is required")
	}

	// Check if it's a skill entry
	entry, ok := t.sched.GetEntry(input.EntryID)
	if !ok {
		return "", fmt.Errorf("unschedule_task: entry not found: %s", input.EntryID)
	}
	if entry.Source == "skill" {
		return "", fmt.Errorf("unschedule_task: cannot remove skill-based schedule %q (managed by skill registry)", input.EntryID)
	}

	title := entry.Title

	if err := t.sched.RemoveEntry(input.EntryID); err != nil {
		return "", fmt.Errorf("unschedule_task: %w", err)
	}

	// Emit removed event
	t.bus.Publish(events.NewTypedEvent(events.SourceScheduler, events.ScheduleRemovedPayload{
		EntryID: input.EntryID,
		Title:   title,
	}))

	result, _ := json.Marshal(map[string]string{
		"entry_id": input.EntryID,
		"status":   "removed",
	})
	return string(result), nil
}

var _ tool.InvokableTool = (*UnscheduleTaskTool)(nil)

// =============================================================================
// list_schedules
// =============================================================================

// ListSchedulesTool lists all schedule entries.
type ListSchedulesTool struct {
	sched *scheduler.Scheduler
}

// NewListSchedulesTool creates a new list_schedules tool.
func NewListSchedulesTool(sched *scheduler.Scheduler) *ListSchedulesTool {
	return &ListSchedulesTool{sched: sched}
}

// ListSchedulesManifest returns the plugin manifest for the list_schedules tool.
func ListSchedulesManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "list_schedules",
		Description: "List scheduled tasks",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "list_schedules",
				Description: "List all active schedule entries. Optionally filter by session ID.",
				Parameters: map[string]ParamSpec{
					"session_id": {
						Type:        "string",
						Description: "Optional session ID to filter by",
					},
				},
			},
		},
	}
}

type listSchedulesInput struct {
	SessionID string `json:"session_id"`
}

type listSchedulesEntry struct {
	ID          string     `json:"id"`
	Source      string     `json:"source"`
	Title       string     `json:"title"`
	CronSpec    string     `json:"cron_spec,omitempty"`
	IntervalSec int        `json:"interval_sec,omitempty"`
	OnEvent     string     `json:"on_event,omitempty"`
	Enabled     bool       `json:"enabled"`
	RunCount    int        `json:"run_count"`
	MaxRuns     int        `json:"max_runs,omitempty"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
}

// Info returns the tool info for Eino registration.
func (t *ListSchedulesTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ListSchedulesManifest().Tools[0]), nil
}

// InvokableRun lists schedule entries.
func (t *ListSchedulesTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input listSchedulesInput
	if argumentsInJSON != "" {
		_ = json.Unmarshal([]byte(argumentsInJSON), &input)
	}

	entries := t.sched.ListEntries()

	var out []listSchedulesEntry
	for _, e := range entries {
		if input.SessionID != "" && e.SessionID != input.SessionID {
			continue
		}

		le := listSchedulesEntry{
			ID:          e.ID,
			Source:      e.Source,
			Title:       e.Title,
			CronSpec:    e.CronSpec,
			IntervalSec: e.IntervalSec,
			Enabled:     e.Enabled,
			RunCount:    e.RunCount,
			MaxRuns:     e.MaxRuns,
			LastRunAt:   e.LastRunAt,
		}
		if e.OnEvent != nil {
			le.OnEvent = e.OnEvent.Event
		}
		out = append(out, le)
	}

	result, err := json.Marshal(map[string]any{
		"count":   len(out),
		"entries": out,
	})
	if err != nil {
		return "", fmt.Errorf("list_schedules: marshal: %w", err)
	}
	return string(result), nil
}

var _ tool.InvokableTool = (*ListSchedulesTool)(nil)
