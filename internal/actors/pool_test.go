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

func TestSubmitDefaultMaxRetries(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 1},
	})

	task := &tasks.Task{
		Title:       "test-default-retries",
		Description: "Should get default MaxRetries",
	}
	if err := pool.Submit(task); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if task.MaxRetries != defaultMaxRetries {
		t.Errorf("MaxRetries: got %d, want %d", task.MaxRetries, defaultMaxRetries)
	}
}

func TestSubmitExplicitMaxRetries(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 1},
	})

	task := &tasks.Task{
		Title:       "test-explicit-retries",
		Description: "Has explicit MaxRetries",
		MaxRetries:  5,
	}
	if err := pool.Submit(task); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if task.MaxRetries != 5 {
		t.Errorf("MaxRetries: got %d, want 5", task.MaxRetries)
	}
}

func TestRequeueForRetry(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 1},
	})

	task := &tasks.Task{
		Title:       "test-requeue",
		Description: "Will be requeued",
		MaxRetries:  3,
	}
	if err := pool.Submit(task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Simulate running
	task.Status = tasks.TaskRunning
	_ = pool.store.Update(task)

	// Requeue
	pool.requeueForRetry(task)

	got, _ := pool.store.Get(task.ID)
	if got.Status != tasks.TaskPending {
		t.Errorf("status: got %s, want pending", got.Status)
	}
	if got.RetryCount != 1 {
		t.Errorf("RetryCount: got %d, want 1", got.RetryCount)
	}
}

func TestRequeueForRetry_ExceedsMax(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 1},
	})

	task := &tasks.Task{
		Title:       "test-max-retries",
		Description: "Will exceed retries",
		MaxRetries:  1,
		RetryCount:  1, // already at max
	}
	if err := pool.Submit(task); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	pool.requeueForRetry(task)

	got, _ := pool.store.Get(task.ID)
	if got.Status != tasks.TaskFailed {
		t.Errorf("status: got %s, want failed", got.Status)
	}
}

func TestProviderCooldown(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"broken": {MaxConcurrent: 1},
		"healthy": {MaxConcurrent: 1},
	})

	// Put "broken" in cooldown
	pool.mu.Lock()
	pool.providerCooldown["broken"] = time.Now().Add(5 * time.Minute)
	pool.mu.Unlock()

	pool.mu.Lock()
	actor := pool.findIdleActor("", nil)
	pool.mu.Unlock()

	if actor == nil {
		t.Fatal("expected to find an idle actor")
	}
	if actor.ProviderName != "healthy" {
		t.Errorf("provider: got %q, want %q", actor.ProviderName, "healthy")
	}
}

func TestProviderCooldownExpired(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"ollama": {MaxConcurrent: 1},
	})

	// Put in expired cooldown
	pool.mu.Lock()
	pool.providerCooldown["ollama"] = time.Now().Add(-1 * time.Second)
	pool.mu.Unlock()

	pool.mu.Lock()
	actor := pool.findIdleActor("", nil)
	pool.mu.Unlock()

	if actor == nil {
		t.Fatal("expected to find actor after cooldown expired")
	}
	if actor.ProviderName != "ollama" {
		t.Errorf("provider: got %q, want %q", actor.ProviderName, "ollama")
	}

	// Verify cooldown was cleaned up
	pool.mu.Lock()
	_, stillCooling := pool.providerCooldown["ollama"]
	pool.mu.Unlock()
	if stillCooling {
		t.Error("expected cooldown entry to be cleaned up after expiry")
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

// --- Inline execution tests ---

func TestShouldInline_SingleActor(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 1},
	})
	if !pool.ShouldInline() {
		t.Error("ShouldInline: got false, want true for single actor")
	}
}

func TestShouldInline_MultiActor(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 2},
	})
	if pool.ShouldInline() {
		t.Error("ShouldInline: got true, want false for 2 actors")
	}
}

func TestShouldInline_MultiProvider(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 1},
		"ollama": {MaxConcurrent: 1},
	})
	if pool.ShouldInline() {
		t.Error("ShouldInline: got true, want false for 2 providers (2 actors total)")
	}
}

func TestExecuteInline_NoModelRegistry(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 1},
	})
	// pool.models is nil (no registry configured)

	task := &tasks.Task{
		Title:       "inline-no-registry",
		Description: "Should fail but still create in store",
	}
	_, err := pool.ExecuteInline(t.Context(), task)
	if err == nil {
		t.Fatal("expected error when model registry is nil")
	}

	// Task should still be in store
	got, storeErr := pool.Store().Get(task.ID)
	if storeErr != nil {
		t.Fatalf("task not in store: %v", storeErr)
	}
	if got.Status != tasks.TaskFailed {
		t.Errorf("status: got %s, want failed", got.Status)
	}
}

func TestExecuteInline_ForcesDisabledAutonomy(t *testing.T) {
	pool := newTestPool(t, map[string]config.ProviderConfig{
		"claude": {MaxConcurrent: 1},
	})

	task := &tasks.Task{
		Title:       "inline-autonomy",
		Description: "Should have autonomy forced to disabled",
		Config: tasks.TaskConfig{
			AutonomyLevel: tasks.AutonomySupervised,
		},
	}
	// This will fail (no model registry) but we can check the task config
	_, _ = pool.ExecuteInline(t.Context(), task)

	if task.Config.AutonomyLevel != tasks.AutonomyDisabled {
		t.Errorf("autonomy: got %q, want %q", task.Config.AutonomyLevel, tasks.AutonomyDisabled)
	}

	// Also verify the task in store has disabled autonomy
	got, err := pool.Store().Get(task.ID)
	if err != nil {
		t.Fatalf("task not in store: %v", err)
	}
	if got.Config.AutonomyLevel != tasks.AutonomyDisabled {
		t.Errorf("store autonomy: got %q, want %q", got.Config.AutonomyLevel, tasks.AutonomyDisabled)
	}
}
