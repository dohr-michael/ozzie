package tasks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/dohr-michael/ozzie/internal/core/brain"
	"github.com/dohr-michael/ozzie/internal/core/events"
)

// ErrPreempted is returned when a task is preempted by a higher-priority request.
var ErrPreempted = errors.New("task preempted")

// TaskRunner executes a single task using an ephemeral agent.
type TaskRunner struct {
	task  *Task
	store Store
	bus   events.EventBus

	runnerFactory   brain.RunnerFactory
	modelName       string
	toolLookup      brain.ToolLookup
	skillRunner     SkillExecutor
	preemptionCheck func() bool
	middlewares     []any                 // opaque, passed to RunnerFactory
	retriever       brain.MemoryRetriever // pre-task memory retrieval (optional)
	tier            brain.ModelTier       // model tier for prompt adaptation
	promptPrefix    string                // overlay-specific prompt prefix
	perms           ToolPermissionsSeeder // for seeding pre-approved tools (optional)
	clientFacing    bool                  // inject persona into sub-agent instruction
	persona         string                // persona text (from LoadPersona)
}

// TaskRunnerConfig holds dependencies for creating a TaskRunner.
type TaskRunnerConfig struct {
	Store           Store
	Bus             events.EventBus
	RunnerFactory   brain.RunnerFactory
	ModelName       string                // provider/model name for CreateRunner
	ToolLookup      brain.ToolLookup
	SkillRunner     SkillExecutor
	PreemptionCheck func() bool
	Middlewares     []any                 // opaque middlewares for sub-agents (e.g. filesystem, reduction)
	Retriever       brain.MemoryRetriever // pre-task memory retrieval (optional)
	Tier            brain.ModelTier       // model tier for prompt adaptation
	PromptPrefix    string                // overlay-specific prompt prefix (optional)
	Perms           ToolPermissionsSeeder // for seeding pre-approved tools (optional)
	ClientFacing    bool                  // inject persona into sub-agent instruction
	Persona         string                // persona text (from LoadPersona)
}

// NewTaskRunner creates a runner for a specific task.
func NewTaskRunner(task *Task, cfg TaskRunnerConfig) *TaskRunner {
	return &TaskRunner{
		task:            task,
		store:           cfg.Store,
		bus:             cfg.Bus,
		runnerFactory:   cfg.RunnerFactory,
		modelName:       cfg.ModelName,
		toolLookup:      cfg.ToolLookup,
		skillRunner:     cfg.SkillRunner,
		preemptionCheck: cfg.PreemptionCheck,
		middlewares:     cfg.Middlewares,
		retriever:       cfg.Retriever,
		tier:            cfg.Tier,
		promptPrefix:    cfg.PromptPrefix,
		perms:           cfg.Perms,
		clientFacing:    cfg.ClientFacing,
		persona:         cfg.Persona,
	}
}

// taskMaxIterations gives async tasks more room for ReAct loops than the default (20).
const taskMaxIterations = 30

// Run executes the task to completion or failure.
func (r *TaskRunner) Run(ctx context.Context) error {
	// Mark context as autonomous + carry session ID for tool permissions
	ctx = events.WithAutonomous(ctx)
	ctx = events.ContextWithTaskID(ctx, r.task.ID)
	if r.task.SessionID != "" {
		ctx = events.ContextWithSessionID(ctx, r.task.SessionID)
	}
	if r.task.Config.WorkDir != "" {
		ctx = events.ContextWithWorkDir(ctx, r.task.Config.WorkDir)
	}
	if len(r.task.Config.Env) > 0 {
		ctx = events.ContextWithTaskEnv(ctx, r.task.Config.Env)
	}
	if len(r.task.Config.ToolConstraints) > 0 {
		ctx = events.ContextWithToolConstraints(ctx, r.task.Config.ToolConstraints)
	}

	// Seed pre-approved tools into permissions (from schedule or submit_task)
	if r.perms != nil && r.task.SessionID != "" && len(r.task.Config.ApprovedTools) > 0 {
		for _, toolName := range r.task.Config.ApprovedTools {
			r.perms.AllowForSession(r.task.SessionID, toolName)
		}
	}

	task := r.task
	startedAt := time.Now()

	// Mark as running
	task.Status = TaskRunning
	task.StartedAt = &startedAt
	if err := r.store.Update(task); err != nil {
		return fmt.Errorf("update task running: %w", err)
	}

	r.bus.Publish(events.NewTypedEventWithSession(events.SourceTask, events.TaskStartedPayload{
		TaskID:       task.ID,
		Title:        task.Title,
		ActorID:      task.ActorID,
		ProviderName: task.ProviderName,
	}, task.SessionID))

	// Skill shortcut: execute directly without agent reasoning
	if task.Config.Skill != "" && r.skillRunner != nil {
		return r.runSkillStep(ctx, task, startedAt)
	}

	return r.runSingleStep(ctx, task, startedAt)
}

// isPreempted checks whether a preemption has been requested.
func (r *TaskRunner) isPreempted() bool {
	return r.preemptionCheck != nil && r.preemptionCheck()
}

// preemptTask re-queues a preempted task as pending.
func (r *TaskRunner) preemptTask(task *Task) error {
	task.Status = TaskPending
	task.StartedAt = nil
	if err := r.store.Update(task); err != nil {
		return fmt.Errorf("update task preempted: %w", err)
	}

	_ = r.store.AppendCheckpoint(task.ID, Checkpoint{
		Ts:      time.Now(),
		Type:    "preempted",
		Summary: "Task preempted by higher-priority request",
	})

	return ErrPreempted
}

func (r *TaskRunner) runSingleStep(ctx context.Context, task *Task, startedAt time.Time) error {
	var tools []brain.Tool
	if len(task.Config.Tools) > 0 {
		tools = r.toolLookup.ToolsByNames(task.Config.Tools)
	}

	depContext := buildDependencyContext(r.store, task.DependsOn)
	memoryContext := r.buildMemoryContext(ctx)
	instruction := r.prefixedInstruction(fmt.Sprintf("Execute the following task.\n\nTitle: %s\nDescription: %s%s%s%s",
		task.Title, task.Description, formatContextBlock(task.Config), depContext, memoryContext))
	if r.clientFacing && r.persona != "" {
		instruction = r.persona + "\n\n" + instruction
	}

	toolNames := make([]string, len(tools))
	for i, t := range tools {
		if info, _ := t.Info(ctx); info != nil {
			toolNames[i] = info.Name
		}
	}
	slog.Info("task agent setup",
		"task_id", task.ID,
		"actor_id", task.ActorID,
		"provider", task.ProviderName,
		"tools", toolNames,
		"instruction_len", len(instruction),
	)

	runner, err := r.runnerFactory.CreateRunner(ctx, r.modelName, instruction, tools,
		brain.WithMaxIterations(taskMaxIterations),
		brain.WithMiddlewares(r.middlewares),
		brain.WithPreemptionCheck(r.isPreempted),
	)
	if err != nil {
		// Model unavailable: don't fail task, let actor pool handle retry
		var unavail *brain.ErrModelUnavailable
		if errors.As(err, &unavail) {
			return err
		}
		return r.failTask(task, startedAt, fmt.Errorf("create agent: %w", err))
	}

	messages := []brain.Message{
		{Role: brain.RoleUser, Content: task.Description},
	}

	output, err := runner.Run(ctx, messages)
	if err != nil {
		// Model unavailable: don't fail task, let actor pool handle retry
		var unavail *brain.ErrModelUnavailable
		if errors.As(err, &unavail) {
			return err
		}
		if errors.Is(err, brain.ErrRunnerPreempted) {
			return r.preemptTask(task)
		}
		return r.failTask(task, startedAt, err)
	}

	return r.completeTask(task, startedAt, output)
}

// runSkillStep executes a skill directly, bypassing agent reasoning.
func (r *TaskRunner) runSkillStep(ctx context.Context, task *Task, startedAt time.Time) error {
	vars := map[string]string{"request": task.Description}
	output, err := r.skillRunner.RunSkill(ctx, task.Config.Skill, vars)
	if err != nil {
		return r.failTask(task, startedAt, fmt.Errorf("skill %s: %w", task.Config.Skill, err))
	}
	return r.completeTask(task, startedAt, output)
}

func (r *TaskRunner) completeTask(task *Task, startedAt time.Time, output string) error {
	now := time.Now()
	task.Status = TaskCompleted
	task.CompletedAt = &now
	task.Progress.Percentage = 100
	task.Result = &TaskResult{
		OutputPath: "output.md",
	}
	if err := r.store.Update(task); err != nil {
		return fmt.Errorf("update task completed: %w", err)
	}

	if output != "" {
		_ = r.store.WriteOutput(task.ID, output)
	}

	r.bus.Publish(events.NewTypedEventWithSession(events.SourceTask, events.TaskCompletedPayload{
		TaskID:        task.ID,
		Title:         task.Title,
		ActorID:       task.ActorID,
		ProviderName:  task.ProviderName,
		OutputSummary: truncate(output, 200),
		Duration:      now.Sub(startedAt),
	}, task.SessionID))

	return nil
}

func (r *TaskRunner) failTask(task *Task, startedAt time.Time, taskErr error) error {
	now := time.Now()
	task.Status = TaskFailed
	task.CompletedAt = &now
	task.Result = &TaskResult{
		Error: taskErr.Error(),
	}
	willRetry := task.RetryCount < task.MaxRetries

	if err := r.store.Update(task); err != nil {
		slog.Error("update task failed", "error", err, "task_id", task.ID)
	}

	_ = r.store.AppendCheckpoint(task.ID, Checkpoint{
		Ts:      now,
		Type:    "failed",
		Summary: taskErr.Error(),
	})

	r.bus.Publish(events.NewTypedEventWithSession(events.SourceTask, events.TaskFailedPayload{
		TaskID:       task.ID,
		Title:        task.Title,
		ActorID:      task.ActorID,
		ProviderName: task.ProviderName,
		Error:        taskErr.Error(),
		RetryCount:   task.RetryCount,
		WillRetry:    willRetry,
	}, task.SessionID))

	_ = startedAt // used in event Duration if needed
	return taskErr
}

// formatContextBlock builds a structured instruction block from task config.
func formatContextBlock(cfg TaskConfig) string {
	var b strings.Builder
	if cfg.WorkDir != "" {
		b.WriteString("\n\n## Execution Context\n")
		fmt.Fprintf(&b, "Working directory: %s\n", cfg.WorkDir)
		b.WriteString("Use this path — do NOT invent paths or assume defaults.\n")
	}
	if len(cfg.Env) > 0 {
		b.WriteString("\n\n## Environment Variables\n")
		for _, k := range slices.Sorted(maps.Keys(cfg.Env)) {
			fmt.Fprintf(&b, "- %s=%s\n", k, cfg.Env[k])
		}
	}
	return b.String()
}

// maxMemoryContextLen is the maximum total length of the memory context block.
const maxMemoryContextLen = 2000

// buildMemoryContext retrieves relevant memories for the task and formats them
// as an instruction block. Single retrieval per task at startup — not per LLM call.
func (r *TaskRunner) buildMemoryContext(ctx context.Context) string {
	if r.retriever == nil {
		return ""
	}
	query := r.task.Title + " " + r.task.Description
	if len(query) > 500 {
		query = query[:500]
	}
	var tags []string
	if len(r.task.Tags) > 0 {
		tags = r.task.Tags
	}

	limit := 5
	maxLen := maxMemoryContextLen
	if r.tier == brain.TierSmall {
		limit = 2
		maxLen = 800
	}

	memories, err := r.retriever.Retrieve(ctx, query, tags, limit)
	if err != nil || len(memories) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## Relevant Memories\n")
	b.WriteString("Past learnings that may be relevant to this task:\n")
	total := 0
	for _, m := range memories {
		line := fmt.Sprintf("- **[%s] %s**: %s\n", m.Entry.Type, m.Entry.Title, m.Content)
		if total+len(line) > maxLen {
			break
		}
		b.WriteString(line)
		total += len(line)
	}
	return b.String()
}

// maxDependencyOutputLen is the maximum length of a single dependency output
// injected into the instruction. Prevents context window overflow.
const maxDependencyOutputLen = 1000

// buildDependencyContext reads outputs from completed dependency tasks
// and formats them as an instruction block for the sub-agent.
func buildDependencyContext(store Store, dependsOn []string) string {
	return buildDependencyContextWithLimit(store, dependsOn, maxDependencyOutputLen)
}

// buildDependencyContextWithLimit is like buildDependencyContext but with a configurable output limit.
func buildDependencyContextWithLimit(store Store, dependsOn []string, maxOutputLen int) string {
	if len(dependsOn) == 0 {
		return ""
	}

	var b strings.Builder
	found := false
	for _, depID := range dependsOn {
		dep, err := store.Get(depID)
		if err != nil {
			continue
		}
		output, err := store.ReadOutput(depID)
		if err != nil || output == "" {
			if !found {
				b.WriteString("\n\n## Completed Dependency Tasks\n")
				found = true
			}
			fmt.Fprintf(&b, "\n### %s (%s) — %s\n", dep.Title, dep.ID, dep.Status)
			if dep.Result != nil && dep.Result.Error != "" {
				fmt.Fprintf(&b, "Error: %s\n", dep.Result.Error)
			} else {
				b.WriteString("(no output captured)\n")
			}
			continue
		}

		if !found {
			b.WriteString("\n\n## Completed Dependency Tasks\n")
			b.WriteString("These tasks ran before yours. Their output describes what was done.\n")
			b.WriteString("Use this to understand existing state — do NOT redo their work.\n")
			found = true
		}

		fmt.Fprintf(&b, "\n### %s (%s)\n", dep.Title, dep.ID)
		if len(output) > maxOutputLen {
			output = output[:maxOutputLen] + "\n... (truncated)"
		}
		b.WriteString(output)
		b.WriteString("\n")
	}

	return b.String()
}

// prefixedInstruction prepends the overlay's prompt prefix (if any) to the instruction.
func (r *TaskRunner) prefixedInstruction(instruction string) string {
	if r.promptPrefix == "" {
		return instruction
	}
	return r.promptPrefix + "\n\n" + instruction
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// NewTaskExecutorFactory returns a brain.TaskExecutorFactory that creates TaskRunner instances.
func NewTaskExecutorFactory() brain.TaskExecutorFactory {
	return func(task *brain.Task, cfg brain.TaskExecutorConfig) brain.TaskExecutor {
		return NewTaskRunner(task, TaskRunnerConfig{
			Store:           cfg.Store,
			Bus:             cfg.Bus,
			RunnerFactory:   cfg.RunnerFactory,
			ModelName:       cfg.ModelName,
			ToolLookup:      cfg.ToolLookup,
			SkillRunner:     cfg.SkillRunner,
			PreemptionCheck: cfg.PreemptionCheck,
			Middlewares:     cfg.Middlewares,
			Retriever:       cfg.Retriever,
			Tier:            cfg.Tier,
			PromptPrefix:    cfg.PromptPrefix,
			Perms:           cfg.Perms,
			ClientFacing:    cfg.ClientFacing,
			Persona:         cfg.Persona,
		})
	}
}
