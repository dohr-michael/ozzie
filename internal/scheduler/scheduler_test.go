package scheduler

import (
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/actors"
	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/tasks"
)

// newTestBus creates a bus for testing.
func newTestBus() *events.Bus {
	return events.NewBus(64)
}

// newTestPool creates a minimal actor pool backed by a temp store.
func newTestPool(t *testing.T, bus *events.Bus) *actors.ActorPool {
	t.Helper()
	dir := t.TempDir()
	store := tasks.NewFileStore(dir)
	return actors.NewActorPool(actors.ActorPoolConfig{
		Providers: map[string]config.ProviderConfig{
			"test": {MaxConcurrent: 1},
		},
		Store: store,
		Bus:   bus,
	})
}

func TestScheduler_LoadEntries(t *testing.T) {
	bus := newTestBus()
	defer bus.Close()

	pool := newTestPool(t, bus)

	skillInfos := []SkillScheduleInfo{
		{Name: "cron-skill", Cron: "*/5 * * * *"},
		{Name: "event-skill", OnEvent: &EventTrigger{Event: "task.completed"}},
	}

	s := New(Config{Pool: pool, Bus: bus, Skills: skillInfos})
	s.Start()
	defer s.Stop()

	entries := s.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Verify cron entry
	var foundCron, foundEvent bool
	for _, e := range entries {
		if e.SkillName == "cron-skill" && e.Cron != nil {
			foundCron = true
		}
		if e.SkillName == "event-skill" && e.OnEvent != nil {
			foundEvent = true
		}
	}
	if !foundCron {
		t.Fatal("expected cron-skill entry with cron expression")
	}
	if !foundEvent {
		t.Fatal("expected event-skill entry with event trigger")
	}
}

func TestScheduler_EventTrigger(t *testing.T) {
	bus := newTestBus()
	defer bus.Close()

	pool := newTestPool(t, bus)
	pool.Start()
	defer pool.Stop()

	skillInfos := []SkillScheduleInfo{
		{Name: "on-complete", OnEvent: &EventTrigger{Event: "task.completed"}},
	}

	s := New(Config{Pool: pool, Bus: bus, Skills: skillInfos})
	s.Start()
	defer s.Stop()

	// Subscribe to schedule trigger events
	triggerCh, unsub := bus.SubscribeChan(4, events.EventScheduleTrigger)
	defer unsub()

	// Publish a task.completed event
	bus.Publish(events.NewTypedEvent(events.SourceTask, events.TaskCompletedPayload{
		TaskID: "task_abc",
		Title:  "test task",
	}))

	// Wait for trigger
	select {
	case e := <-triggerCh:
		payload, ok := events.GetScheduleTriggerPayload(e)
		if !ok {
			t.Fatal("failed to extract schedule trigger payload")
		}
		if payload.SkillName != "on-complete" {
			t.Fatalf("expected skill %q, got %q", "on-complete", payload.SkillName)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for schedule trigger event")
	}
}

func TestScheduler_CooldownPreventsDoubleTrigger(t *testing.T) {
	bus := newTestBus()
	defer bus.Close()

	pool := newTestPool(t, bus)
	pool.Start()
	defer pool.Stop()

	skillInfos := []SkillScheduleInfo{
		{Name: "cooldown-test", OnEvent: &EventTrigger{Event: "task.completed"}},
	}

	s := New(Config{Pool: pool, Bus: bus, Skills: skillInfos})
	s.Start()
	defer s.Stop()

	// Count trigger events
	triggerCh, unsub := bus.SubscribeChan(8, events.EventScheduleTrigger)
	defer unsub()

	// Fire two events rapidly
	bus.Publish(events.NewTypedEvent(events.SourceTask, events.TaskCompletedPayload{
		TaskID: "task_1", Title: "t1",
	}))

	// Wait for first trigger
	select {
	case <-triggerCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first trigger")
	}

	// Fire second event immediately (should be blocked by cooldown)
	bus.Publish(events.NewTypedEvent(events.SourceTask, events.TaskCompletedPayload{
		TaskID: "task_2", Title: "t2",
	}))

	// Second trigger should NOT arrive
	select {
	case <-triggerCh:
		t.Fatal("expected cooldown to prevent second trigger")
	case <-time.After(200 * time.Millisecond):
		// Good — no second trigger
	}
}

func TestScheduler_AddEntry_Dynamic(t *testing.T) {
	bus := newTestBus()
	defer bus.Close()

	pool := newTestPool(t, bus)
	pool.Start()
	defer pool.Stop()

	store := NewScheduleStore(t.TempDir())

	s := New(Config{Pool: pool, Bus: bus, Store: store})
	s.Start()
	defer s.Stop()

	// Subscribe to schedule trigger events
	triggerCh, unsub := bus.SubscribeChan(4, events.EventScheduleTrigger)
	defer unsub()

	entry := &ScheduleEntry{
		Source:      "dynamic",
		Title:       "git check",
		Description: "Check git status periodically",
		IntervalSec: 1, // 1s for test speed
		CooldownSec: 0,
		Enabled:     true,
		TaskTemplate: &TaskTemplate{
			Title:       "git status check",
			Description: "Run git status and report changes",
			Tools:       []string{"cmd"},
		},
	}

	// IntervalSec < 5 should fail
	if err := s.AddEntry(entry); err == nil {
		t.Fatal("expected error for interval < 5s")
	}

	// Fix interval
	entry.IntervalSec = 5
	entry.CooldownSec = 1
	if err := s.AddEntry(entry); err != nil {
		t.Fatalf("add entry: %v", err)
	}

	if entry.ID == "" {
		t.Fatal("expected ID to be generated")
	}

	// Verify it appears in entries
	entries := s.ListEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Verify persistence
	persisted, err := store.List()
	if err != nil {
		t.Fatalf("store list: %v", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("expected 1 persisted entry, got %d", len(persisted))
	}

	// Wait for interval trigger (checkIntervals runs every second)
	select {
	case e := <-triggerCh:
		payload, ok := events.GetScheduleTriggerPayload(e)
		if !ok {
			t.Fatal("failed to extract payload")
		}
		if payload.EntryID != entry.ID {
			t.Fatalf("expected entry ID %q, got %q", entry.ID, payload.EntryID)
		}
		if payload.Trigger != "interval" {
			t.Fatalf("expected trigger %q, got %q", "interval", payload.Trigger)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for interval trigger")
	}
}

func TestScheduler_RemoveEntry(t *testing.T) {
	bus := newTestBus()
	defer bus.Close()

	pool := newTestPool(t, bus)
	store := NewScheduleStore(t.TempDir())

	s := New(Config{Pool: pool, Bus: bus, Store: store})
	s.Start()
	defer s.Stop()

	entry := &ScheduleEntry{
		Source:      "dynamic",
		Title:       "to remove",
		Description: "will be removed",
		IntervalSec: 60,
		Enabled:     true,
		TaskTemplate: &TaskTemplate{
			Title:       "test",
			Description: "test",
		},
	}

	if err := s.AddEntry(entry); err != nil {
		t.Fatalf("add: %v", err)
	}

	if err := s.RemoveEntry(entry.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if len(s.ListEntries()) != 0 {
		t.Fatal("expected 0 entries after remove")
	}

	// Verify removed from store
	persisted, _ := store.List()
	if len(persisted) != 0 {
		t.Fatal("expected 0 persisted entries after remove")
	}

	// Remove non-existent should error
	if err := s.RemoveEntry("sched_nonexistent"); err == nil {
		t.Fatal("expected error for non-existent entry")
	}
}

func TestScheduler_MaxRuns(t *testing.T) {
	bus := newTestBus()
	defer bus.Close()

	pool := newTestPool(t, bus)
	pool.Start()
	defer pool.Stop()

	store := NewScheduleStore(t.TempDir())

	s := New(Config{Pool: pool, Bus: bus, Store: store})
	s.Start()
	defer s.Stop()

	triggerCh, unsub := bus.SubscribeChan(8, events.EventScheduleTrigger)
	defer unsub()

	entry := &ScheduleEntry{
		Source:      "dynamic",
		Title:       "max-2",
		Description: "stops after 2 runs",
		IntervalSec: 5,
		CooldownSec: 1,
		MaxRuns:     2,
		Enabled:     true,
		TaskTemplate: &TaskTemplate{
			Title:       "limited task",
			Description: "limited",
		},
	}

	if err := s.AddEntry(entry); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Wait for 2 triggers
	for i := 0; i < 2; i++ {
		select {
		case <-triggerCh:
		case <-time.After(15 * time.Second):
			t.Fatalf("timeout waiting for trigger %d", i+1)
		}
	}

	// Third trigger should not come (entry disabled)
	select {
	case <-triggerCh:
		t.Fatal("expected entry to be disabled after max runs")
	case <-time.After(8 * time.Second):
		// Good
	}

	// Verify entry is disabled
	se, ok := s.GetEntry(entry.ID)
	if !ok {
		t.Fatal("entry not found")
	}
	if se.Enabled {
		t.Fatal("expected entry to be disabled")
	}
	if se.RunCount != 2 {
		t.Fatalf("expected run count 2, got %d", se.RunCount)
	}
}

func TestScheduler_LoadPersistedEntries(t *testing.T) {
	bus := newTestBus()
	defer bus.Close()

	pool := newTestPool(t, bus)
	storeDir := t.TempDir()
	store := NewScheduleStore(storeDir)

	// Pre-persist an entry
	entry := &ScheduleEntry{
		ID:          "sched_pre1",
		Source:      "dynamic",
		Title:       "pre-existing",
		Description: "loaded from disk",
		IntervalSec: 60,
		CooldownSec: 60,
		Enabled:     true,
		TaskTemplate: &TaskTemplate{
			Title:       "persisted task",
			Description: "from disk",
		},
	}
	if err := store.Create(entry); err != nil {
		t.Fatalf("pre-persist: %v", err)
	}

	// Create scheduler with same store — should load the entry
	s := New(Config{Pool: pool, Bus: bus, Store: store})
	s.Start()
	defer s.Stop()

	entries := s.ListEntries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry loaded from store, got %d", len(entries))
	}
	if entries[0].ID != "sched_pre1" {
		t.Fatalf("expected pre-existing entry, got %q", entries[0].ID)
	}
}

func TestScheduler_NoStoreBackwardsCompat(t *testing.T) {
	bus := newTestBus()
	defer bus.Close()

	pool := newTestPool(t, bus)

	// No store — should work fine (original behavior)
	s := New(Config{Pool: pool, Bus: bus})
	s.Start()
	defer s.Stop()

	if len(s.ListEntries()) != 0 {
		t.Fatal("expected 0 entries with no skills and no store")
	}
}
