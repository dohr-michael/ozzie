package actors

import (
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/tasks"
)

func newTestPool(t *testing.T, providers map[string]config.ProviderConfig) *ActorPool {
	t.Helper()
	store := tasks.NewFileStore(t.TempDir())
	bus := events.NewBus(64)
	t.Cleanup(bus.Close)

	return NewActorPool(ActorPoolConfig{
		Providers: providers,
		Store:     store,
		Bus:       bus,
	})
}

func TestActorPoolStartStop(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 2, Tags: []string{"coding"}},
	})

	pool.Start()
	time.Sleep(100 * time.Millisecond)
	pool.Stop()

	if len(pool.actors) != 2 {
		t.Errorf("actors: got %d, want 2", len(pool.actors))
	}
}

func TestActorPoolSubmit(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 1},
	})

	task := &tasks.Task{
		Title:       "test-submit",
		Description: "A test task",
	}
	if err := pool.Submit(task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if task.ID == "" {
		t.Fatal("expected task ID to be set")
	}
	if task.Status != tasks.TaskPending {
		t.Errorf("status: got %s, want pending", task.Status)
	}

	// Verify in store
	got, err := pool.Store().Get(task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "test-submit" {
		t.Errorf("title: got %q, want %q", got.Title, "test-submit")
	}
}

func TestActorPoolCancel(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 1},
	})

	task := &tasks.Task{
		Title:       "test-cancel",
		Description: "Will be cancelled",
	}
	if err := pool.Submit(task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if err := pool.Cancel(task.ID, "testing"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	got, err := pool.Store().Get(task.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != tasks.TaskCancelled {
		t.Errorf("status: got %s, want cancelled", got.Status)
	}
}

func TestActorCreation(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 2, Tags: []string{"coding", "chat"}},
		"local":  {MaxConcurrent: 3, Tags: []string{"fast", "privacy"}},
	})

	if len(pool.actors) != 5 {
		t.Fatalf("actors: got %d, want 5", len(pool.actors))
	}

	claudeCount := 0
	localCount := 0
	for _, a := range pool.actors {
		switch a.ProviderName {
		case "claude":
			claudeCount++
		case "local":
			localCount++
		}
	}

	if claudeCount != 2 {
		t.Errorf("claude actors: got %d, want 2", claudeCount)
	}
	if localCount != 3 {
		t.Errorf("local actors: got %d, want 3", localCount)
	}
}

func TestActorMatchesTags(t *testing.T) {
	actor := &Actor{
		Tags: []string{"coding", "chat", "fast"},
	}

	tests := []struct {
		tags []string
		want bool
	}{
		{nil, true},
		{[]string{}, true},
		{[]string{"coding"}, true},
		{[]string{"coding", "chat"}, true},
		{[]string{"coding", "chat", "fast"}, true},
		{[]string{"privacy"}, false},
		{[]string{"coding", "privacy"}, false},
	}

	for _, tt := range tests {
		got := actor.MatchesTags(tt.tags)
		if got != tt.want {
			t.Errorf("MatchesTags(%v): got %v, want %v", tt.tags, got, tt.want)
		}
	}
}

func TestAcquireInteractiveIdle(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 2},
	})

	actor, err := pool.AcquireInteractive("claude")
	if err != nil {
		t.Fatalf("AcquireInteractive: %v", err)
	}
	if actor == nil {
		t.Fatal("expected non-nil actor")
	}
	if actor.Status != ActorBusy {
		t.Errorf("status: got %s, want busy", actor.Status)
	}
	if actor.ProviderName != "claude" {
		t.Errorf("provider: got %s, want claude", actor.ProviderName)
	}

	// Release and verify
	pool.Release(actor)
	if actor.Status != ActorIdle {
		t.Errorf("after release: got %s, want idle", actor.Status)
	}
}

func TestAcquireInteractiveNoProvider(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 1},
	})

	// Acquire the only slot
	a1, err := pool.AcquireInteractive("claude")
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	// Release
	pool.Release(a1)

	// Try non-existent provider — no actors match
	_, err = pool.AcquireInteractive("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent provider")
	}
}

func TestPriorityRank(t *testing.T) {
	if priorityRank(tasks.PriorityLow) >= priorityRank(tasks.PriorityNormal) {
		t.Error("low should rank below normal")
	}
	if priorityRank(tasks.PriorityNormal) >= priorityRank(tasks.PriorityHigh) {
		t.Error("normal should rank below high")
	}
}
