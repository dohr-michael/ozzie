package scheduler

import (
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/events"
)

func makeEvent(eventType events.EventType, source events.EventSource, payload map[string]any) events.Event {
	return events.Event{
		ID:        "test-1",
		Type:      eventType,
		Timestamp: time.Now(),
		Source:    source,
		Payload:   payload,
	}
}

func TestMatchEvent_BasicMatch(t *testing.T) {
	trigger := &EventTrigger{Event: "task.completed"}
	e := makeEvent("task.completed", events.SourceTask, nil)

	if !MatchEvent(e, trigger) {
		t.Fatal("expected match for matching event type")
	}
}

func TestMatchEvent_TypeMismatch(t *testing.T) {
	trigger := &EventTrigger{Event: "task.completed"}
	e := makeEvent("task.failed", events.SourceTask, nil)

	if MatchEvent(e, trigger) {
		t.Fatal("expected no match for different event type")
	}
}

func TestMatchEvent_NilTrigger(t *testing.T) {
	e := makeEvent("task.completed", events.SourceTask, nil)

	if MatchEvent(e, nil) {
		t.Fatal("expected no match for nil trigger")
	}
}

func TestMatchEvent_RejectsSchedulerSource(t *testing.T) {
	trigger := &EventTrigger{Event: "task.completed"}
	e := makeEvent("task.completed", events.SourceScheduler, nil)

	if MatchEvent(e, trigger) {
		t.Fatal("expected no match for scheduler-sourced event (loop prevention)")
	}
}

func TestMatchEvent_FilterMatch(t *testing.T) {
	trigger := &EventTrigger{
		Event:  "skill.completed",
		Filter: map[string]string{"skill_name": "deploy"},
	}
	e := makeEvent("skill.completed", events.SourceSkill, map[string]any{
		"skill_name": "deploy",
		"output":     "success",
	})

	if !MatchEvent(e, trigger) {
		t.Fatal("expected match when filter matches payload")
	}
}

func TestMatchEvent_FilterMismatch(t *testing.T) {
	trigger := &EventTrigger{
		Event:  "skill.completed",
		Filter: map[string]string{"skill_name": "deploy"},
	}
	e := makeEvent("skill.completed", events.SourceSkill, map[string]any{
		"skill_name": "build",
	})

	if MatchEvent(e, trigger) {
		t.Fatal("expected no match when filter value differs")
	}
}

func TestMatchEvent_FilterMissingKey(t *testing.T) {
	trigger := &EventTrigger{
		Event:  "skill.completed",
		Filter: map[string]string{"skill_name": "deploy"},
	}
	e := makeEvent("skill.completed", events.SourceSkill, map[string]any{})

	if MatchEvent(e, trigger) {
		t.Fatal("expected no match when filter key is missing from payload")
	}
}
