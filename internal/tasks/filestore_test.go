package tasks

import (
	"testing"
	"time"
)

func TestFileStoreCRUD(t *testing.T) {
	store := NewFileStore(t.TempDir())

	// Create
	task := &Task{
		Title:       "Test task",
		Description: "A test task",
		Status:      TaskPending,
		Priority:    PriorityNormal,
	}
	if err := store.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty task ID")
	}

	// Get
	got, err := store.Get(task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Test task" {
		t.Errorf("Title: got %q, want %q", got.Title, "Test task")
	}
	if got.Status != TaskPending {
		t.Errorf("Status: got %q, want %q", got.Status, TaskPending)
	}

	// Update
	got.Status = TaskRunning
	now := time.Now()
	got.StartedAt = &now
	if err := store.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got2, err := store.Get(task.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got2.Status != TaskRunning {
		t.Errorf("Status after update: got %q, want %q", got2.Status, TaskRunning)
	}

	// Delete
	if err := store.Delete(task.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = store.Get(task.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestFileStoreList(t *testing.T) {
	store := NewFileStore(t.TempDir())

	// Create tasks with different statuses and sessions
	tasks := []struct {
		title     string
		status    TaskStatus
		sessionID string
	}{
		{"task-a", TaskPending, "sess_aaa"},
		{"task-b", TaskRunning, "sess_bbb"},
		{"task-c", TaskCompleted, "sess_aaa"},
	}

	for _, tc := range tasks {
		task := &Task{
			Title:     tc.title,
			Status:    tc.status,
			SessionID: tc.sessionID,
			Priority:  PriorityNormal,
		}
		if err := store.Create(task); err != nil {
			t.Fatalf("Create %s: %v", tc.title, err)
		}
		// Small sleep to ensure distinct UpdatedAt
		time.Sleep(10 * time.Millisecond)
	}

	// List all
	all, err := store.List(ListFilter{})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("List all: got %d, want 3", len(all))
	}

	// Filter by status
	pending, err := store.List(ListFilter{Status: TaskPending})
	if err != nil {
		t.Fatalf("List pending: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("List pending: got %d, want 1", len(pending))
	}

	// Filter by session
	sessA, err := store.List(ListFilter{SessionID: "sess_aaa"})
	if err != nil {
		t.Fatalf("List session: %v", err)
	}
	if len(sessA) != 2 {
		t.Errorf("List session sess_aaa: got %d, want 2", len(sessA))
	}
}

func TestFileStoreCheckpoints(t *testing.T) {
	store := NewFileStore(t.TempDir())

	task := &Task{
		Title:    "checkpoint-task",
		Status:   TaskRunning,
		Priority: PriorityNormal,
	}
	if err := store.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Append checkpoints
	cp1 := Checkpoint{Ts: time.Now(), StepID: "step_1", Type: "started", Summary: "Step 1 started"}
	cp2 := Checkpoint{Ts: time.Now(), StepID: "step_1", Type: "completed", Summary: "Step 1 done"}

	if err := store.AppendCheckpoint(task.ID, cp1); err != nil {
		t.Fatalf("AppendCheckpoint 1: %v", err)
	}
	if err := store.AppendCheckpoint(task.ID, cp2); err != nil {
		t.Fatalf("AppendCheckpoint 2: %v", err)
	}

	// Load checkpoints
	cps, err := store.LoadCheckpoints(task.ID)
	if err != nil {
		t.Fatalf("LoadCheckpoints: %v", err)
	}
	if len(cps) != 2 {
		t.Fatalf("LoadCheckpoints: got %d, want 2", len(cps))
	}
	if cps[0].StepID != "step_1" || cps[0].Type != "started" {
		t.Errorf("Checkpoint 0: got %+v", cps[0])
	}
}

func TestFileStoreOutput(t *testing.T) {
	store := NewFileStore(t.TempDir())

	task := &Task{
		Title:    "output-task",
		Status:   TaskRunning,
		Priority: PriorityNormal,
	}
	if err := store.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Write output
	content := "# Task Output\n\nHello world!"
	if err := store.WriteOutput(task.ID, content); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	// Read output
	got, err := store.ReadOutput(task.ID)
	if err != nil {
		t.Fatalf("ReadOutput: %v", err)
	}
	if got != content {
		t.Errorf("ReadOutput: got %q, want %q", got, content)
	}

	// Read output of non-existent task
	empty, err := store.ReadOutput("nonexistent")
	if err != nil {
		t.Fatalf("ReadOutput nonexistent: %v", err)
	}
	if empty != "" {
		t.Errorf("ReadOutput nonexistent: got %q, want empty", empty)
	}
}

func TestFileStoreLoadCheckpointsEmpty(t *testing.T) {
	store := NewFileStore(t.TempDir())

	// No file exists
	cps, err := store.LoadCheckpoints("nonexistent")
	if err != nil {
		t.Fatalf("LoadCheckpoints nonexistent: %v", err)
	}
	if cps != nil {
		t.Errorf("expected nil, got %v", cps)
	}
}
