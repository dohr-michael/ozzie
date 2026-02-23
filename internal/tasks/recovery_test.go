package tasks

import (
	"testing"
)

func TestRecoverTasks(t *testing.T) {
	store := NewFileStore(t.TempDir())

	// Create tasks in various states
	running1 := &Task{Title: "running-1", Status: TaskRunning, Priority: PriorityNormal}
	running2 := &Task{Title: "running-2", Status: TaskRunning, Priority: PriorityNormal}
	pending := &Task{Title: "pending-1", Status: TaskPending, Priority: PriorityNormal}
	completed := &Task{Title: "completed-1", Status: TaskCompleted, Priority: PriorityNormal}

	for _, task := range []*Task{running1, running2, pending, completed} {
		if err := store.Create(task); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	recovered, err := RecoverTasks(store)
	if err != nil {
		t.Fatalf("RecoverTasks: %v", err)
	}
	if recovered != 2 {
		t.Errorf("recovered: got %d, want 2", recovered)
	}

	// Verify running tasks are now pending
	t1, _ := store.Get(running1.ID)
	if t1.Status != TaskPending {
		t.Errorf("running-1 status: got %s, want pending", t1.Status)
	}

	t2, _ := store.Get(running2.ID)
	if t2.Status != TaskPending {
		t.Errorf("running-2 status: got %s, want pending", t2.Status)
	}

	// Verify checkpoints were written
	cps, _ := store.LoadCheckpoints(running1.ID)
	if len(cps) != 1 {
		t.Errorf("recovery checkpoints: got %d, want 1", len(cps))
	}
	if len(cps) > 0 && cps[0].Type != "recovery" {
		t.Errorf("checkpoint type: got %s, want recovery", cps[0].Type)
	}

	// Verify other tasks unchanged
	tp, _ := store.Get(pending.ID)
	if tp.Status != TaskPending {
		t.Errorf("pending-1 status: got %s, want pending", tp.Status)
	}

	tc, _ := store.Get(completed.ID)
	if tc.Status != TaskCompleted {
		t.Errorf("completed-1 status: got %s, want completed", tc.Status)
	}
}
