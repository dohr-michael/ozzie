package tasks

import (
	"time"
)

// RecoverTasks resets all running and suspended tasks to pending state after a crash.
// Should be called on gateway startup before starting the actor pool.
func RecoverTasks(store Store) (int, error) {
	running, err := store.List(ListFilter{Status: TaskRunning})
	if err != nil {
		return 0, err
	}

	suspended, err := store.List(ListFilter{Status: TaskSuspended})
	if err != nil {
		return 0, err
	}

	toRecover := append(running, suspended...)

	recovered := 0
	for _, t := range toRecover {
		// Keep tasks waiting for user reply suspended — don't auto-resume
		if t.WaitingForReply {
			continue
		}

		t.Status = TaskPending
		t.StartedAt = nil
		t.SuspendedAt = nil
		if err := store.Update(t); err != nil {
			continue
		}

		_ = store.AppendCheckpoint(t.ID, Checkpoint{
			Ts:      time.Now(),
			Type:    "recovery",
			Summary: "Task recovered after gateway restart",
		})

		recovered++
	}

	return recovered, nil
}
