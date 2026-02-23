package plugins

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dohr-michael/ozzie/internal/actors"
	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/scheduler"
	"github.com/dohr-michael/ozzie/internal/tasks"
)

func newScheduleTestDeps(t *testing.T) (*scheduler.Scheduler, *events.Bus) {
	t.Helper()
	bus := events.NewBus(64)
	t.Cleanup(func() { bus.Close() })

	store := tasks.NewFileStore(t.TempDir())
	pool := actors.NewActorPool(actors.ActorPoolConfig{
		Providers: map[string]config.ProviderConfig{
			"test": {MaxConcurrent: 1},
		},
		Store: store,
		Bus:   bus,
	})

	schedStore := scheduler.NewScheduleStore(t.TempDir())
	sched := scheduler.New(scheduler.Config{
		Pool:   pool,
		Bus:    bus,
		Store:  schedStore,
	})
	sched.Start()
	t.Cleanup(func() { sched.Stop() })

	return sched, bus
}

func TestScheduleTaskTool_Create(t *testing.T) {
	sched, bus := newScheduleTestDeps(t)
	tool := NewScheduleTaskTool(sched, bus)

	input := `{"title":"git monitor","description":"check git changes","interval":"30s","tools":["cmd"]}`
	result, err := tool.InvokableRun(context.Background(), input)
	if err != nil {
		t.Fatalf("schedule_task: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out["status"] != "created" {
		t.Fatalf("expected status created, got %v", out["status"])
	}
	entryID, ok := out["entry_id"].(string)
	if !ok || entryID == "" {
		t.Fatal("expected entry_id in result")
	}

	// Verify entry exists in scheduler
	entries := sched.ListEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].IntervalSec != 30 {
		t.Fatalf("expected interval 30, got %d", entries[0].IntervalSec)
	}
}

func TestScheduleTaskTool_CronTrigger(t *testing.T) {
	sched, bus := newScheduleTestDeps(t)
	tool := NewScheduleTaskTool(sched, bus)

	input := `{"title":"hourly check","description":"run hourly","cron":"0 * * * *"}`
	result, err := tool.InvokableRun(context.Background(), input)
	if err != nil {
		t.Fatalf("schedule_task cron: %v", err)
	}

	var out map[string]any
	_ = json.Unmarshal([]byte(result), &out)
	if out["status"] != "created" {
		t.Fatalf("expected status created, got %v", out["status"])
	}
}

func TestScheduleTaskTool_EventTrigger(t *testing.T) {
	sched, bus := newScheduleTestDeps(t)
	tool := NewScheduleTaskTool(sched, bus)

	input := `{"title":"on complete","description":"react to task completion","on_event":"task.completed","cooldown":"10s"}`
	result, err := tool.InvokableRun(context.Background(), input)
	if err != nil {
		t.Fatalf("schedule_task event: %v", err)
	}

	var out map[string]any
	_ = json.Unmarshal([]byte(result), &out)
	if out["status"] != "created" {
		t.Fatalf("expected status created, got %v", out["status"])
	}
}

func TestScheduleTaskTool_Validation(t *testing.T) {
	sched, bus := newScheduleTestDeps(t)
	tool := NewScheduleTaskTool(sched, bus)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"missing title", `{"description":"d","interval":"30s"}`, "title is required"},
		{"missing desc", `{"title":"t","interval":"30s"}`, "description is required"},
		{"no trigger", `{"title":"t","description":"d"}`, "one of cron, interval, or on_event is required"},
		{"multiple triggers", `{"title":"t","description":"d","cron":"* * * * *","interval":"30s"}`, "mutually exclusive"},
		{"invalid interval", `{"title":"t","description":"d","interval":"nope"}`, "invalid interval"},
		{"invalid cron", `{"title":"t","description":"d","cron":"bad"}`, "parse cron"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.InvokableRun(context.Background(), tt.input)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %q", tt.want, err.Error())
			}
		})
	}
}

func TestUnscheduleTaskTool(t *testing.T) {
	sched, bus := newScheduleTestDeps(t)
	scheduleTool := NewScheduleTaskTool(sched, bus)
	unscheduleTool := NewUnscheduleTaskTool(sched, bus)

	// Create an entry first
	input := `{"title":"to remove","description":"will be removed","interval":"60s"}`
	result, err := scheduleTool.InvokableRun(context.Background(), input)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var created map[string]any
	_ = json.Unmarshal([]byte(result), &created)
	entryID := created["entry_id"].(string)

	// Remove it
	removeInput, _ := json.Marshal(map[string]string{"entry_id": entryID})
	result, err = unscheduleTool.InvokableRun(context.Background(), string(removeInput))
	if err != nil {
		t.Fatalf("unschedule: %v", err)
	}

	var removed map[string]any
	_ = json.Unmarshal([]byte(result), &removed)
	if removed["status"] != "removed" {
		t.Fatalf("expected status removed, got %v", removed["status"])
	}

	// Verify gone
	if len(sched.ListEntries()) != 0 {
		t.Fatal("expected 0 entries after unschedule")
	}
}

func TestUnscheduleTaskTool_NotFound(t *testing.T) {
	sched, bus := newScheduleTestDeps(t)
	tool := NewUnscheduleTaskTool(sched, bus)

	input := `{"entry_id":"sched_nonexistent"}`
	_, err := tool.InvokableRun(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for non-existent entry")
	}
}

func TestListSchedulesTool(t *testing.T) {
	sched, bus := newScheduleTestDeps(t)
	scheduleTool := NewScheduleTaskTool(sched, bus)
	listTool := NewListSchedulesTool(sched)

	// Create two entries
	_, _ = scheduleTool.InvokableRun(context.Background(),
		`{"title":"first","description":"d1","interval":"30s"}`)
	_, _ = scheduleTool.InvokableRun(context.Background(),
		`{"title":"second","description":"d2","cron":"*/5 * * * *"}`)

	result, err := listTool.InvokableRun(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	var out map[string]any
	_ = json.Unmarshal([]byte(result), &out)
	count := out["count"].(float64)
	if count != 2 {
		t.Fatalf("expected 2 entries, got %v", count)
	}
}

func TestListSchedulesTool_EmptyArgs(t *testing.T) {
	sched, _ := newScheduleTestDeps(t)
	listTool := NewListSchedulesTool(sched)

	result, err := listTool.InvokableRun(context.Background(), "")
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}

	var out map[string]any
	_ = json.Unmarshal([]byte(result), &out)
	count := out["count"].(float64)
	if count != 0 {
		t.Fatalf("expected 0 entries, got %v", count)
	}
}
