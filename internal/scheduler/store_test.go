package scheduler

import (
	"testing"
)

func TestScheduleStore_CRUD(t *testing.T) {
	dir := t.TempDir()
	store := NewScheduleStore(dir)

	// Create
	entry := &ScheduleEntry{
		Source:      "dynamic",
		Title:       "test schedule",
		Description: "check git status",
		IntervalSec: 30,
		CooldownSec: 30,
		Enabled:     true,
	}

	if err := store.Create(entry); err != nil {
		t.Fatalf("create: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("expected ID to be generated")
	}
	if entry.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}

	// Get
	got, err := store.Get(entry.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "test schedule" {
		t.Fatalf("expected title %q, got %q", "test schedule", got.Title)
	}
	if got.IntervalSec != 30 {
		t.Fatalf("expected interval 30, got %d", got.IntervalSec)
	}

	// Update
	got.RunCount = 5
	if err := store.Update(got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, err := store.Get(entry.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got2.RunCount != 5 {
		t.Fatalf("expected run count 5, got %d", got2.RunCount)
	}

	// Delete
	if err := store.Delete(entry.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err = store.Get(entry.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestScheduleStore_List(t *testing.T) {
	dir := t.TempDir()
	store := NewScheduleStore(dir)

	// Create two entries
	e1 := &ScheduleEntry{
		Source:      "dynamic",
		SessionID:   "sess_a",
		Title:       "first",
		Description: "first schedule",
		IntervalSec: 10,
		Enabled:     true,
	}
	e2 := &ScheduleEntry{
		Source:      "dynamic",
		SessionID:   "sess_b",
		Title:       "second",
		Description: "second schedule",
		CronSpec:    "*/5 * * * *",
		Enabled:     true,
	}

	if err := store.Create(e1); err != nil {
		t.Fatalf("create e1: %v", err)
	}
	if err := store.Create(e2); err != nil {
		t.Fatalf("create e2: %v", err)
	}

	// List all
	all, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}

	// List by session
	sessA, err := store.ListBySession("sess_a")
	if err != nil {
		t.Fatalf("list by session: %v", err)
	}
	if len(sessA) != 1 {
		t.Fatalf("expected 1 entry for sess_a, got %d", len(sessA))
	}
	if sessA[0].Title != "first" {
		t.Fatalf("expected title %q, got %q", "first", sessA[0].Title)
	}
}

func TestScheduleStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewScheduleStore(dir)

	_, err := store.Get("sched_nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent entry")
	}
}
