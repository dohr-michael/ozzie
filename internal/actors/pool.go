package actors

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"

	"github.com/dohr-michael/ozzie/internal/agent"
	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/models"
	"github.com/dohr-michael/ozzie/internal/tasks"
)

// preemptionTimeout is the hard cancel delay after a cooperative preemption request.
const preemptionTimeout = 30 * time.Second

// runningTask tracks a task currently executing on an actor.
type runningTask struct {
	taskID    string
	actor     *Actor
	cancel    context.CancelFunc
	preemptCh chan struct{} // closed to signal cooperative preemption
}

// ActorPool manages LLM capacity slots and task scheduling.
type ActorPool struct {
	mu           sync.Mutex
	actors       []*Actor
	runners      map[string]*runningTask // taskID → running state
	store        tasks.Store
	bus          *events.Bus
	models       *models.Registry
	toolRegistry agent.ToolLookup

	autonomyDefault     string
	maxValidationRounds int
	skillRunner         tasks.SkillExecutor
	taskMiddlewares     []adk.AgentMiddleware

	scheduleCh chan struct{} // wake-up signal for the scheduler
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// ActorPoolConfig holds configuration for building an ActorPool.
type ActorPoolConfig struct {
	Providers           map[string]config.ProviderConfig
	Store               tasks.Store
	Bus                 *events.Bus
	Models              *models.Registry
	ToolRegistry        agent.ToolLookup
	AutonomyDefault     string              // "disabled" | "supervised" | "autonomous"
	MaxValidationRounds int                 // max plan-revise cycles (0 = default 3)
	SkillRunner         tasks.SkillExecutor // optional skill executor for direct skill tasks
	TaskMiddlewares     []adk.AgentMiddleware // middlewares for sub-agents (filesystem, reduction)
}

// NewActorPool creates an ActorPool from provider configurations.
func NewActorPool(cfg ActorPoolConfig) *ActorPool {
	var actors []*Actor

	for name, prov := range cfg.Providers {
		n := prov.MaxConcurrent
		if n <= 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			actors = append(actors, &Actor{
				ID:           fmt.Sprintf("%s-%d", name, i),
				ProviderName: name,
				Tags:         prov.Tags,
				Status:       ActorIdle,
			})
		}
	}

	return &ActorPool{
		actors:              actors,
		runners:             make(map[string]*runningTask),
		store:               cfg.Store,
		bus:                 cfg.Bus,
		models:              cfg.Models,
		toolRegistry:        cfg.ToolRegistry,
		autonomyDefault:     cfg.AutonomyDefault,
		maxValidationRounds: cfg.MaxValidationRounds,
		skillRunner:         cfg.SkillRunner,
		taskMiddlewares:     cfg.TaskMiddlewares,
		scheduleCh:          make(chan struct{}, 1),
	}
}

// Start launches the scheduler loop.
func (p *ActorPool) Start() {
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.wg.Add(1)
	go p.scheduleLoop()
	slog.Info("actor pool started", "actors", len(p.actors))
}

// Stop cancels all running tasks and waits for goroutines to finish.
func (p *ActorPool) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	slog.Info("actor pool stopped")
}

// Store returns the underlying task store.
func (p *ActorPool) Store() tasks.Store {
	return p.store
}

// Submit creates a task and wakes the scheduler.
func (p *ActorPool) Submit(t *tasks.Task) error {
	if t.ID == "" {
		t.ID = tasks.GenerateTaskID()
	}
	if t.Status == "" {
		t.Status = tasks.TaskPending
	}
	if t.Priority == "" {
		t.Priority = tasks.PriorityNormal
	}

	if err := p.store.Create(t); err != nil {
		return err
	}

	p.bus.Publish(events.NewTypedEventWithSession(events.SourceTask, events.TaskCreatedPayload{
		TaskID:      t.ID,
		Title:       t.Title,
		Description: t.Description,
		ParentID:    t.ParentTaskID,
	}, t.SessionID))

	p.wakeScheduler()
	return nil
}

// Cancel cancels a running or pending task.
func (p *ActorPool) Cancel(taskID string, reason string) error {
	p.mu.Lock()
	if rt, ok := p.runners[taskID]; ok {
		rt.cancel()
	}
	p.mu.Unlock()

	task, err := p.store.Get(taskID)
	if err != nil {
		return err
	}

	if task.Status == tasks.TaskCompleted || task.Status == tasks.TaskCancelled {
		return nil
	}

	now := time.Now()
	task.Status = tasks.TaskCancelled
	task.CompletedAt = &now
	if err := p.store.Update(task); err != nil {
		return err
	}

	_ = p.store.AppendCheckpoint(taskID, tasks.Checkpoint{
		Ts:      now,
		Type:    "cancelled",
		Summary: reason,
	})

	p.bus.Publish(events.NewTypedEventWithSession(events.SourceTask, events.TaskCancelledPayload{
		TaskID: taskID,
		Reason: reason,
	}, task.SessionID))

	// Cancel child tasks recursively
	children, err := p.store.List(tasks.ListFilter{ParentID: taskID})
	if err != nil {
		return nil // best effort
	}
	for _, child := range children {
		if child.Status == tasks.TaskPending || child.Status == tasks.TaskRunning {
			_ = p.Cancel(child.ID, "parent cancelled")
		}
	}

	return nil
}

// AcquireInteractive acquires a capacity slot for interactive (user-facing) use.
// If all slots for the provider are busy, it preempts the lowest-priority task.
func (p *ActorPool) AcquireInteractive(providerName string) (*Actor, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to find an idle actor for this provider
	if actor := p.findIdleActor(providerName, nil); actor != nil {
		actor.Status = ActorBusy
		actor.CurrentTask = "_interactive"
		return actor, nil
	}

	// No idle actor — preempt lowest-priority task on this provider
	if actor := p.preemptLowest(providerName); actor != nil {
		return actor, nil
	}

	return nil, fmt.Errorf("no actors available for provider %q", providerName)
}

// Release frees a capacity slot and wakes the scheduler.
func (p *ActorPool) Release(actor *Actor) {
	if actor == nil {
		return
	}
	p.mu.Lock()
	actor.Status = ActorIdle
	actor.CurrentTask = ""
	p.mu.Unlock()
	p.wakeScheduler()
}

// wakeScheduler sends a non-blocking signal to the schedule loop.
func (p *ActorPool) wakeScheduler() {
	select {
	case p.scheduleCh <- struct{}{}:
	default:
	}
}

// scheduleLoop is the main scheduler goroutine.
func (p *ActorPool) scheduleLoop() {
	defer p.wg.Done()

	pollTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()

	for {
		p.schedule()

		select {
		case <-p.ctx.Done():
			return
		case <-p.scheduleCh:
		case <-pollTicker.C:
		}
	}
}

// schedule assigns pending/suspended tasks to idle actors.
func (p *ActorPool) schedule() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 1. Resume suspended tasks first (skip tasks waiting for user reply)
	suspended, _ := p.store.List(tasks.ListFilter{Status: tasks.TaskSuspended})
	for _, t := range suspended {
		if t.WaitingForReply {
			continue
		}

		actor := p.findIdleActor("", t.Tags)
		if actor == nil {
			continue
		}
		actor.Status = ActorBusy
		actor.CurrentTask = t.ID

		// Reset suspended state
		t.Status = tasks.TaskPending
		t.SuspendedAt = nil
		_ = p.store.Update(t)

		p.bus.Publish(events.NewTypedEventWithSession(events.SourceTask, events.TaskResumedPayload{
			TaskID: t.ID,
			Title:  t.Title,
		}, t.SessionID))

		p.startTask(t, actor)
	}

	// 2. Assign pending tasks (oldest first, dependencies resolved)
	pending, _ := p.store.List(tasks.ListFilter{Status: tasks.TaskPending})
	// List returns sorted by UpdatedAt DESC, iterate in reverse for oldest first
	for i := len(pending) - 1; i >= 0; i-- {
		t := pending[i]
		if !p.dependenciesResolved(t) {
			continue
		}

		actor := p.findIdleActor("", t.Tags)
		if actor == nil {
			if len(t.Tags) > 0 {
				slog.Warn("no actor matches task tags", "task_id", t.ID, "tags", t.Tags)
			}
			continue
		}
		actor.Status = ActorBusy
		actor.CurrentTask = t.ID

		p.startTask(t, actor)
	}
}

// findIdleActor returns the first idle actor matching the provider (if non-empty) and tags.
// Caller must hold p.mu.
func (p *ActorPool) findIdleActor(providerName string, requiredTags []string) *Actor {
	for _, a := range p.actors {
		if a.Status != ActorIdle {
			continue
		}
		if providerName != "" && a.ProviderName != providerName {
			continue
		}
		if !a.MatchesTags(requiredTags) {
			continue
		}
		return a
	}
	return nil
}

// preemptLowest requests preemption of the lowest-priority task on the given provider.
// Returns the actor that will be freed once the task suspends.
// Caller must hold p.mu.
func (p *ActorPool) preemptLowest(providerName string) *Actor {
	var lowestRT *runningTask
	lowestPriority := priorityRank(tasks.PriorityHigh) + 1

	for _, rt := range p.runners {
		if rt.actor.ProviderName != providerName {
			continue
		}
		task, err := p.store.Get(rt.taskID)
		if err != nil {
			continue
		}
		rank := priorityRank(task.Priority)
		if rank < lowestPriority {
			lowestPriority = rank
			lowestRT = rt
		}
	}

	if lowestRT == nil {
		return nil
	}

	slog.Info("preempting task for interactive use",
		"task_id", lowestRT.taskID, "actor", lowestRT.actor.ID)

	// Signal cooperative preemption
	select {
	case <-lowestRT.preemptCh:
		// already closed
	default:
		close(lowestRT.preemptCh)
	}

	// Hard cancel timeout goroutine
	taskID := lowestRT.taskID
	go func() {
		time.Sleep(preemptionTimeout)
		p.mu.Lock()
		if rt, ok := p.runners[taskID]; ok {
			slog.Warn("hard-cancelling task after preemption timeout", "task_id", taskID)
			rt.cancel()
		}
		p.mu.Unlock()
	}()

	// Wait briefly for the task to suspend (up to 5s before returning the actor)
	actor := lowestRT.actor

	// Release mu while waiting
	p.mu.Unlock()
	deadline := time.After(5 * time.Second)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-deadline:
			p.mu.Lock()
			// Force-take the actor
			actor.Status = ActorBusy
			actor.CurrentTask = "_interactive"
			return actor
		case <-tick.C:
			p.mu.Lock()
			if actor.Status == ActorIdle {
				actor.Status = ActorBusy
				actor.CurrentTask = "_interactive"
				return actor
			}
			p.mu.Unlock()
		}
	}
}

// startTask launches a goroutine to execute a task on an actor.
// Caller must hold p.mu.
func (p *ActorPool) startTask(t *tasks.Task, actor *Actor) {
	taskCtx, taskCancel := context.WithCancel(p.ctx)
	preemptCh := make(chan struct{})

	rt := &runningTask{
		taskID:    t.ID,
		actor:     actor,
		cancel:    taskCancel,
		preemptCh: preemptCh,
	}
	p.runners[t.ID] = rt

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			taskCancel()
			p.mu.Lock()
			delete(p.runners, t.ID)
			actor.Status = ActorIdle
			actor.CurrentTask = ""
			p.mu.Unlock()
			p.wakeScheduler()
		}()

		p.executeTask(taskCtx, t, actor, preemptCh)
	}()
}

// executeTask runs a single task using the TaskRunner.
func (p *ActorPool) executeTask(ctx context.Context, t *tasks.Task, actor *Actor, preemptCh chan struct{}) {
	slog.Info("actor executing task", "actor", actor.ID, "task_id", t.ID, "title", t.Title)

	if p.models == nil {
		slog.Error("no model registry configured", "task_id", t.ID)
		_ = p.failTaskDirect(t, fmt.Errorf("no model registry configured"))
		return
	}

	chatModel, err := p.models.Get(ctx, actor.ProviderName)
	if err != nil {
		slog.Error("get model for task", "error", err, "task_id", t.ID, "provider", actor.ProviderName)
		_ = p.failTaskDirect(t, fmt.Errorf("get model: %w", err))
		return
	}

	preemptionCheck := func() bool {
		select {
		case <-preemptCh:
			return true
		default:
			return false
		}
	}

	runner := tasks.NewTaskRunner(t, tasks.TaskRunnerConfig{
		Store:               p.store,
		Bus:                 p.bus,
		ChatModel:           chatModel,
		ToolRegistry:        p.toolRegistry,
		SkillRunner:         p.skillRunner,
		PreemptionCheck:     preemptionCheck,
		MaxValidationRounds: p.maxValidationRounds,
		Middlewares:         p.taskMiddlewares,
	})

	if err := runner.Run(ctx); err != nil {
		if ctx.Err() != nil {
			slog.Info("task cancelled", "task_id", t.ID)
		} else {
			slog.Error("task failed", "error", err, "task_id", t.ID)
		}
	}
}

func (p *ActorPool) failTaskDirect(t *tasks.Task, taskErr error) error {
	now := time.Now()
	t.Status = tasks.TaskFailed
	t.CompletedAt = &now
	t.Result = &tasks.TaskResult{Error: taskErr.Error()}
	return p.store.Update(t)
}

// dependenciesResolved checks whether all dependencies of a task are completed.
func (p *ActorPool) dependenciesResolved(t *tasks.Task) bool {
	if len(t.DependsOn) == 0 {
		return true
	}
	for _, depID := range t.DependsOn {
		dep, err := p.store.Get(depID)
		if err != nil {
			return false
		}
		if dep.Status != tasks.TaskCompleted {
			return false
		}
	}
	return true
}

// ResumeTask clears WaitingForReply, resets to pending, and wakes the scheduler.
func (p *ActorPool) ResumeTask(taskID string) error {
	task, err := p.store.Get(taskID)
	if err != nil {
		return fmt.Errorf("resume task: %w", err)
	}

	if task.Status != tasks.TaskSuspended {
		return fmt.Errorf("resume task: task %s is not suspended (status: %s)", taskID, task.Status)
	}

	task.WaitingForReply = false
	task.Status = tasks.TaskPending
	task.SuspendedAt = nil
	if err := p.store.Update(task); err != nil {
		return fmt.Errorf("resume task update: %w", err)
	}

	p.bus.Publish(events.NewTypedEventWithSession(events.SourceTask, events.TaskResumedPayload{
		TaskID: task.ID,
		Title:  task.Title,
	}, task.SessionID))

	p.wakeScheduler()
	return nil
}

// priorityRank maps task priority to a numeric rank (lower = less important).
func priorityRank(p tasks.TaskPriority) int {
	switch p {
	case tasks.PriorityLow:
		return 0
	case tasks.PriorityNormal:
		return 1
	case tasks.PriorityHigh:
		return 2
	default:
		return 1
	}
}
