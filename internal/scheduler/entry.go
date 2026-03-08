package scheduler

import (
	"time"

	"github.com/dohr-michael/ozzie/internal/events"
)

// EventTrigger describes an event-based trigger for a schedule entry.
// Mirrors skills.EventTrigger to avoid import cycles (plugins → scheduler → skills → plugins).
type EventTrigger struct {
	Event  string            `json:"event"`
	Filter map[string]string `json:"filter,omitempty"`
}

// ScheduleEntry represents a persistent schedule entry (skill-based or dynamic).
type ScheduleEntry struct {
	ID           string        `json:"id"`
	Source       string        `json:"source"` // "skill" or "dynamic"
	SessionID    string        `json:"session_id,omitempty"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	CronSpec     string        `json:"cron_spec,omitempty"`
	IntervalSec  int           `json:"interval_sec,omitempty"`
	OnEvent      *EventTrigger `json:"on_event,omitempty"`
	TaskTemplate *TaskTemplate `json:"task_template,omitempty"`
	SkillName    string        `json:"skill_name,omitempty"`
	CooldownSec  int           `json:"cooldown_sec"`
	MaxRuns      int           `json:"max_runs,omitempty"`
	RunCount     int           `json:"run_count"`
	Enabled      bool          `json:"enabled"`
	CreatedAt    time.Time     `json:"created_at"`
	LastRunAt    *time.Time    `json:"last_run_at,omitempty"`
}

// TaskTemplate defines the task to create on each trigger of a dynamic schedule.
type TaskTemplate struct {
	Title           string                            `json:"title"`
	Description     string                            `json:"description"`
	Tools           []string                          `json:"tools,omitempty"`
	WorkDir         string                            `json:"work_dir,omitempty"`
	Env             map[string]string                 `json:"env,omitempty"`
	ApprovedTools   []string                          `json:"approved_tools,omitempty"`   // dangerous tools pre-approved for scheduled tasks
	ToolConstraints map[string]*events.ToolConstraint `json:"tool_constraints,omitempty"` // per-tool argument constraints
}

// SkillScheduleInfo carries the scheduling-relevant data from a skill definition.
// Used to decouple the scheduler package from the skills package.
type SkillScheduleInfo struct {
	Name    string
	Cron    string
	OnEvent *EventTrigger
}

