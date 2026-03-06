package tasks

import (
	"time"
)

// RecoverTasks resets all running tasks to pending state after a crash.
// Should be called on gateway startup before starting the actor pool.
func RecoverTasks(store Store) (int, error) {
	running, err := store.List(ListFilter{Status: TaskRunning})
	if err != nil {
		return 0, err
	}

	recovered := 0
	for _, t := range running {
		t.Status = TaskPending
		t.StartedAt = nil
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
