package actors

import (
	"context"
	"errors"
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

// providerCooldownDuration is how long a provider is skipped after returning ErrModelUnavailable.
const providerCooldownDuration = 2 * time.Minute

// defaultMaxRetries is applied to tasks that don't specify MaxRetries.
const defaultMaxRetries = 3

// runningTask tracks a task currently executing on an actor.
type runningTask struct {
	taskID    string
	actor     *Actor
	cancel    context.CancelFunc
	preemptCh chan struct{} // closed to signal cooperative preemption
}

// ActorPool manages LLM capacity slots and task scheduling.
type ActorPool struct {
	mu               sync.Mutex
	actors           []*Actor
	runners          map[string]*runningTask // taskID → running state
	providerCooldown map[string]time.Time    // provider → cooldown expiry
	store            tasks.Store
	bus              *events.Bus
	models           *models.Registry
	toolRegistry     agent.ToolLookup

	skillRunner     tasks.SkillExecutor
	taskMiddlewares     []adk.AgentMiddleware
	retriever           agent.MemoryRetriever       // pre-task memory retrieval (optional)
	perms               tasks.ToolPermissionsSeeder // for seeding pre-approved tools (optional)

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
	SkillRunner         tasks.SkillExecutor         // optional skill executor for direct skill tasks
	TaskMiddlewares     []adk.AgentMiddleware       // middlewares for sub-agents (filesystem, reduction)
	Retriever           agent.MemoryRetriever       // pre-task memory retrieval (optional)
	Perms               tasks.ToolPermissionsSeeder // for seeding pre-approved tools (optional)
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
				Capabilities: prov.Capabilities,
				PromptPrefix: prov.PromptPrefix,
				Status:       ActorIdle,
			})
		}
	}

	return &ActorPool{
		actors:              actors,
		runners:             make(map[string]*runningTask),
		providerCooldown:    make(map[string]time.Time),
		store:               cfg.Store,
		bus:                 cfg.Bus,
		models:              cfg.Models,
		toolRegistry:        cfg.ToolRegistry,
		skillRunner:         cfg.SkillRunner,
		taskMiddlewares:     cfg.TaskMiddlewares,
		retriever:           cfg.Retriever,
		perms:               cfg.Perms,
		scheduleCh:          make(chan struct{}, 1),
	}
}

// Start launches the scheduler loop and subscribes to task completion events.
func (p *ActorPool) Start() {
	p.ctx, p.cancel = context.WithCancel(context.Background())

	// Subscribe to task completed/failed events to wake the scheduler immediately
	// when a dependency might have been resolved.
	ch, unsub := p.bus.SubscribeChan(16, events.EventTaskCompleted, events.EventTaskFailed)
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer unsub()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ch:
				p.wakeScheduler()
			}
		}
	}()

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
	if t.Status == "" {
		t.Status = tasks.TaskPending
	}
	if t.Priority == "" {
		t.Priority = tasks.PriorityNormal
	}
	if t.MaxRetries == 0 {
		t.MaxRetries = defaultMaxRetries
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
	if actor := p.findIdleActor(providerName, nil, nil); actor != nil {
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

// schedule assigns pending tasks to idle actors.
// It also cancels tasks whose dependencies can never be satisfied (failed/cancelled).
func (p *ActorPool) schedule() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Assign pending tasks (oldest first, dependencies resolved)
	pending, _ := p.store.List(tasks.ListFilter{Status: tasks.TaskPending})
	// List returns sorted by UpdatedAt DESC, iterate in reverse for oldest first
	for i := len(pending) - 1; i >= 0; i-- {
		t := pending[i]

		// Cancel tasks with unresolvable dependencies (dep failed/cancelled)
		if reason := p.unresolvableDep(t); reason != "" {
			slog.Info("cancelling task with unresolvable dependency", "task_id", t.ID, "reason", reason)
			now := time.Now()
			t.Status = tasks.TaskCancelled
			t.CompletedAt = &now
			_ = p.store.Update(t)
			_ = p.store.AppendCheckpoint(t.ID, tasks.Checkpoint{
				Ts:      now,
				Type:    "cancelled",
				Summary: reason,
			})
			p.bus.Publish(events.NewTypedEventWithSession(events.SourceTask, events.TaskCancelledPayload{
				TaskID: t.ID,
				Reason: reason,
			}, t.SessionID))
			continue
		}

		if !p.dependenciesResolved(t) {
			continue
		}

		actor := p.findIdleActor("", t.Tags, t.Config.RequiredCapabilities)
		if actor == nil {
			if len(t.Tags) > 0 || len(t.Config.RequiredCapabilities) > 0 {
				slog.Warn("no actor matches task requirements", "task_id", t.ID, "tags", t.Tags, "capabilities", t.Config.RequiredCapabilities)
			}
			continue
		}
		actor.Status = ActorBusy
		actor.CurrentTask = t.ID

		p.startTask(t, actor)
	}
}

// findIdleActor returns the first idle actor matching the provider (if non-empty), tags, and capabilities.
// Actors whose provider is in cooldown are skipped.
// Caller must hold p.mu.
func (p *ActorPool) findIdleActor(providerName string, requiredTags []string, requiredCaps []string) *Actor {
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
		if !a.MatchesCapabilities(requiredCaps) {
			continue
		}
		// Skip providers in cooldown
		if expiry, ok := p.providerCooldown[a.ProviderName]; ok {
			if time.Now().Before(expiry) {
				continue
			}
			delete(p.providerCooldown, a.ProviderName)
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
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		timer := time.NewTimer(preemptionTimeout)
		defer timer.Stop()
		select {
		case <-timer.C:
			p.mu.Lock()
			if rt, ok := p.runners[taskID]; ok {
				slog.Warn("hard-cancelling task after preemption timeout", "task_id", taskID)
				rt.cancel()
			}
			p.mu.Unlock()
		case <-p.ctx.Done():
			return
		}
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
			// Hard-cancel the task before force-taking the actor
			if rt, ok := p.runners[lowestRT.taskID]; ok {
				rt.cancel()
			}
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
	slog.Info("actor executing task", "actor", actor.ID, "task_id", t.ID, "title", t.Title, "provider", actor.ProviderName)

	// Tag the task with actor info for traceability
	t.ActorID = actor.ID
	t.ProviderName = actor.ProviderName

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

	tier := p.models.ProviderTier(actor.ProviderName)

	runner := tasks.NewTaskRunner(t, tasks.TaskRunnerConfig{
		Store:           p.store,
		Bus:             p.bus,
		ChatModel:       chatModel,
		ToolRegistry:    p.toolRegistry,
		SkillRunner:     p.skillRunner,
		PreemptionCheck: preemptionCheck,
		Middlewares:     p.taskMiddlewares,
		Retriever:       p.retriever,
		Tier:            tier,
		PromptPrefix:    actor.PromptPrefix,
		Perms:           p.perms,
	})

	if err := runner.Run(ctx); err != nil {
		var unavail *models.ErrModelUnavailable
		if errors.As(err, &unavail) {
			// Mark provider as temporarily down and re-queue
			p.mu.Lock()
			p.providerCooldown[actor.ProviderName] = time.Now().Add(providerCooldownDuration)
			p.mu.Unlock()
			slog.Warn("model unavailable, will retry on another actor",
				"provider", actor.ProviderName, "task_id", t.ID, "error", unavail)
			p.requeueForRetry(t)
			return
		}
		if ctx.Err() != nil {
			slog.Info("task cancelled", "task_id", t.ID)
		} else {
			slog.Error("task failed", "error", err, "task_id", t.ID)
		}
	}
}

// requeueForRetry re-queues a task for retry on a different actor.
func (p *ActorPool) requeueForRetry(t *tasks.Task) {
	t.RetryCount++
	if t.RetryCount > t.MaxRetries {
		slog.Error("task exceeded max retries", "task_id", t.ID, "retries", t.RetryCount)
		_ = p.failTaskDirect(t, fmt.Errorf("model unavailable after %d retries", t.RetryCount))
		return
	}
	t.Status = tasks.TaskPending
	t.CompletedAt = nil
	t.Result = nil
	_ = p.store.Update(t)
	slog.Info("task re-queued for retry", "task_id", t.ID, "retry", t.RetryCount, "max", t.MaxRetries)
}

func (p *ActorPool) failTaskDirect(t *tasks.Task, taskErr error) error {
	now := time.Now()
	t.Status = tasks.TaskFailed
	t.CompletedAt = &now
	t.Result = &tasks.TaskResult{Error: taskErr.Error()}
	return p.store.Update(t)
}

// unresolvableDep returns a non-empty reason if any dependency of the task has
// failed or been cancelled (meaning this task can never be scheduled).
func (p *ActorPool) unresolvableDep(t *tasks.Task) string {
	for _, depID := range t.DependsOn {
		dep, err := p.store.Get(depID)
		if err != nil {
			continue // don't cancel on store errors, just skip
		}
		if dep.Status == tasks.TaskFailed {
			return fmt.Sprintf("dependency %s (%s) failed", depID, dep.Title)
		}
		if dep.Status == tasks.TaskCancelled {
			return fmt.Sprintf("dependency %s (%s) cancelled", depID, dep.Title)
		}
	}
	return ""
}

// dependenciesResolved checks whether all dependencies of a task are completed.
func (p *ActorPool) dependenciesResolved(t *tasks.Task) bool {
	if len(t.DependsOn) == 0 {
		return true
	}
	for _, depID := range t.DependsOn {
		dep, err := p.store.Get(depID)
		if err != nil {
			slog.Debug("dependency check: store error", "task_id", t.ID, "dep_id", depID, "error", err)
			return false
		}
		if dep.Status != tasks.TaskCompleted {
			slog.Debug("dependency not resolved", "task_id", t.ID, "dep_id", depID, "dep_status", dep.Status)
			return false
		}
	}
	return true
}

// ShouldInline returns true when the pool has exactly one actor, meaning async
// submission would deadlock (the single actor is occupied by the caller).
func (p *ActorPool) ShouldInline() bool {
	return len(p.actors) == 1
}

// ExecuteInline runs a task synchronously in the caller's goroutine.
// It applies the same defaults as Submit, creates the task in the store,
// and delegates to a TaskRunner.
func (p *ActorPool) ExecuteInline(ctx context.Context, t *tasks.Task) (string, error) {
	// Apply defaults (same as Submit)
	if t.Status == "" {
		t.Status = tasks.TaskPending
	}
	if t.Priority == "" {
		t.Priority = tasks.PriorityNormal
	}
	if t.MaxRetries == 0 {
		t.MaxRetries = defaultMaxRetries
	}

	// Persist to store (needed for dependency reads)
	if err := p.store.Create(t); err != nil {
		return "", fmt.Errorf("inline create: %w", err)
	}

	p.bus.Publish(events.NewTypedEventWithSession(events.SourceTask, events.TaskCreatedPayload{
		TaskID:      t.ID,
		Title:       t.Title,
		Description: t.Description,
		ParentID:    t.ParentTaskID,
	}, t.SessionID))

	// Resolve model from the first (only) actor's provider
	if p.models == nil {
		_ = p.failTaskDirect(t, fmt.Errorf("no model registry configured"))
		return "", fmt.Errorf("inline: no model registry configured")
	}

	actor := p.actors[0]
	chatModel, err := p.models.Get(ctx, actor.ProviderName)
	if err != nil {
		_ = p.failTaskDirect(t, fmt.Errorf("get model: %w", err))
		return "", fmt.Errorf("inline get model: %w", err)
	}

	tier := p.models.ProviderTier(actor.ProviderName)

	runner := tasks.NewTaskRunner(t, tasks.TaskRunnerConfig{
		Store:           p.store,
		Bus:             p.bus,
		ChatModel:       chatModel,
		ToolRegistry:    p.toolRegistry,
		SkillRunner:     p.skillRunner,
		PreemptionCheck: func() bool { return false },
		Middlewares:     p.taskMiddlewares,
		Retriever:       p.retriever,
		Tier:            tier,
		PromptPrefix:    actor.PromptPrefix,
		Perms:           p.perms,
	})

	if err := runner.Run(ctx); err != nil {
		// Task is already marked failed in store by the runner
		return "", fmt.Errorf("inline run: %w", err)
	}

	output, _ := p.store.ReadOutput(t.ID)
	return output, nil
}

var _ tasks.InlineExecutor = (*ActorPool)(nil)

// AvailableActors returns a deduplicated summary of actor tags and capabilities,
// grouped by provider name.
func (p *ActorPool) AvailableActors() []tasks.ActorInfo {
	p.mu.Lock()
	defer p.mu.Unlock()

	seen := make(map[string]struct{})
	var infos []tasks.ActorInfo

	for _, a := range p.actors {
		if _, ok := seen[a.ProviderName]; ok {
			continue
		}
		seen[a.ProviderName] = struct{}{}
		infos = append(infos, tasks.ActorInfo{
			ProviderName: a.ProviderName,
			Tags:         a.Tags,
			Capabilities: a.Capabilities,
		})
	}
	return infos
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
