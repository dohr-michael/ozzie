package tasks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/agent"
	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/models"
)

// ErrPreempted is returned when a task is preempted by a higher-priority request.
var ErrPreempted = errors.New("task preempted")

// ErrSelfSuspend is returned when a task requests self-suspension (validation).
var ErrSelfSuspend = errors.New("task self-suspended for validation")

// TaskRunner executes a single task using an ephemeral agent.
type TaskRunner struct {
	task  *Task
	store Store
	bus   *events.Bus

	chatModel           model.ToolCallingChatModel
	toolRegistry        agent.ToolLookup
	skillRunner         SkillExecutor
	preemptionCheck     func() bool
	maxValidationRounds int
	middlewares         []adk.AgentMiddleware

	selfSuspendCh chan events.ValidationRequest // side channel for self-suspension
}

// TaskRunnerConfig holds dependencies for creating a TaskRunner.
type TaskRunnerConfig struct {
	Store               Store
	Bus                 *events.Bus
	ChatModel           model.ToolCallingChatModel
	ToolRegistry        agent.ToolLookup
	SkillRunner         SkillExecutor
	PreemptionCheck     func() bool
	MaxValidationRounds int                    // max plan-revise cycles (0 = default 3)
	Middlewares         []adk.AgentMiddleware   // middlewares for sub-agents (e.g. filesystem, reduction)
}

// NewTaskRunner creates a runner for a specific task.
func NewTaskRunner(task *Task, cfg TaskRunnerConfig) *TaskRunner {
	maxRounds := cfg.MaxValidationRounds
	if maxRounds == 0 {
		maxRounds = 3
	}

	// Prepend tool recovery middleware so sub-agents can self-correct on tool errors
	recoveryMw := adk.AgentMiddleware{
		WrapToolCall: agent.NewToolRecoveryMiddleware(agent.ToolRecoveryConfig{}),
	}
	mws := make([]adk.AgentMiddleware, 0, 1+len(cfg.Middlewares))
	mws = append(mws, recoveryMw)
	mws = append(mws, cfg.Middlewares...)

	return &TaskRunner{
		task:                task,
		store:               cfg.Store,
		bus:                 cfg.Bus,
		chatModel:           cfg.ChatModel,
		toolRegistry:        cfg.ToolRegistry,
		skillRunner:         cfg.SkillRunner,
		preemptionCheck:     cfg.PreemptionCheck,
		maxValidationRounds: maxRounds,
		middlewares:         mws,
		selfSuspendCh:       make(chan events.ValidationRequest, 1),
	}
}

// taskMaxIterations gives async tasks more room for ReAct loops than the default (20).
const taskMaxIterations = 30

// Run executes the task to completion or failure.
func (r *TaskRunner) Run(ctx context.Context) error {
	// Mark context as autonomous + carry session ID for tool permissions
	ctx = events.WithAutonomous(ctx)
	ctx = events.ContextWithTaskID(ctx, r.task.ID)
	ctx = events.ContextWithValidationCh(ctx, r.selfSuspendCh)
	if r.task.SessionID != "" {
		ctx = events.ContextWithSessionID(ctx, r.task.SessionID)
	}
	if r.task.Config.WorkDir != "" {
		ctx = events.ContextWithWorkDir(ctx, r.task.Config.WorkDir)
	}
	if len(r.task.Config.Env) > 0 {
		ctx = events.ContextWithTaskEnv(ctx, r.task.Config.Env)
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
		TaskID: task.ID,
		Title:  task.Title,
	}, task.SessionID))

	// Skill shortcut: execute directly without agent reasoning
	if task.Config.Skill != "" && r.skillRunner != nil {
		return r.runSkillStep(ctx, task, startedAt)
	}

	// If no plan, run as single step
	if task.Plan == nil || len(task.Plan.Steps) == 0 {
		return r.runSingleStep(ctx, task, startedAt)
	}

	return r.runPlanSteps(ctx, task, startedAt)
}

// isPreempted checks whether a preemption has been requested.
func (r *TaskRunner) isPreempted() bool {
	return r.preemptionCheck != nil && r.preemptionCheck()
}

// suspendTask transitions the task to suspended state and publishes an event.
func (r *TaskRunner) suspendTask(task *Task, reason string) error {
	return r.suspendTaskWithPayload(task, reason, "", "")
}

// suspendTaskWithPayload suspends a task with optional plan content and validation token.
func (r *TaskRunner) suspendTaskWithPayload(task *Task, reason, planContent, token string) error {
	now := time.Now()
	task.Status = TaskSuspended
	task.SuspendedAt = &now
	task.SuspendCount++

	if err := r.store.Update(task); err != nil {
		return fmt.Errorf("update task suspended: %w", err)
	}

	_ = r.store.AppendCheckpoint(task.ID, Checkpoint{
		Ts:      now,
		Type:    "suspended",
		Summary: reason,
	})

	r.bus.Publish(events.NewTypedEventWithSession(events.SourceTask, events.TaskSuspendedPayload{
		TaskID:       task.ID,
		Title:        task.Title,
		Reason:       reason,
		SuspendCount: task.SuspendCount,
		PlanContent:  planContent,
		Token:        token,
	}, task.SessionID))

	if task.SuspendCount > 3 {
		slog.Warn("task suspended repeatedly", "task_id", task.ID, "count", task.SuspendCount)
	}

	return nil
}

func (r *TaskRunner) runSingleStep(ctx context.Context, task *Task, startedAt time.Time) error {
	switch task.Config.AutonomyLevel {
	case AutonomySupervised:
		// Supervised coordinator: explore → plan → validate → execute
		mailbox, _ := r.store.LoadMailbox(task.ID)
		switch lastFeedbackStatus(mailbox) {
		case "approved":
			return r.runCoordinatorExecution(ctx, task, startedAt, mailbox)
		case "revise":
			return r.runCoordinatorPlanning(ctx, task, startedAt)
		default:
			return r.runCoordinatorPlanning(ctx, task, startedAt)
		}
	case AutonomyAutonomous:
		// Autonomous coordinator: explore → plan → execute (no validation)
		return r.runCoordinatorAutonomous(ctx, task, startedAt)
	}

	// Standard task — single agent
	var tools []tool.InvokableTool
	if len(task.Config.Tools) > 0 {
		tools = r.toolRegistry.ToolsByNames(task.Config.Tools)
	}

	depContext := buildDependencyContext(r.store, task.DependsOn)
	mailboxContext := buildMailboxContext(r.store, task.ID)
	instruction := fmt.Sprintf("Execute the following task.\n\nTitle: %s\nDescription: %s%s%s%s",
		task.Title, task.Description, formatContextBlock(task.Config), depContext, mailboxContext)

	slog.Debug("task agent instruction",
		"task_id", task.ID,
		"autonomy", task.Config.AutonomyLevel,
		"instruction_length", len(instruction),
		"instruction", instruction,
	)

	runner, err := agent.NewAgentBuffered(ctx, r.chatModel, instruction, tools, r.middlewares, agent.AgentOptions{MaxIterations: taskMaxIterations})
	if err != nil {
		return r.failTask(task, startedAt, fmt.Errorf("create agent: %w", err))
	}

	messages := []*schema.Message{
		{Role: schema.User, Content: task.Description},
	}

	output, err := r.consumeRunnerOutput(ctx, runner, messages)
	if err != nil {
		// Model unavailable: don't fail task, let actor pool handle retry
		var unavail *models.ErrModelUnavailable
		if errors.As(err, &unavail) {
			return err
		}
		if errors.Is(err, ErrPreempted) {
			return r.suspendTask(task, "preempted during single step")
		}
		var sse *selfSuspendError
		if errors.As(err, &sse) {
			return r.selfSuspendTask(task, sse.request, sse.explorationContext)
		}
		return r.failTask(task, startedAt, err)
	}

	return r.completeTask(task, startedAt, output)
}

// runCoordinatorPlanning runs Phase 1 of a coordinator task: explore, plan, request_validation.
func (r *TaskRunner) runCoordinatorPlanning(ctx context.Context, task *Task, startedAt time.Time) error {
	// Check max validation rounds — count existing "request" messages
	mailbox, _ := r.store.LoadMailbox(task.ID)
	requestCount := 0
	for _, msg := range mailbox {
		if msg.Type == "request" {
			requestCount++
		}
	}
	if requestCount >= r.maxValidationRounds {
		return r.failTask(task, startedAt, fmt.Errorf("coordinator: exceeded max validation rounds (%d)", r.maxValidationRounds))
	}

	var tools []tool.InvokableTool
	if len(task.Config.Tools) > 0 {
		tools = r.toolRegistry.ToolsByNames(task.Config.Tools)
	}

	depContext := buildDependencyContext(r.store, task.DependsOn)
	mailboxContext := buildMailboxContext(r.store, task.ID)
	instruction := agent.LoadCoordinatorPrompt()
	instruction += fmt.Sprintf("\n\n## Task\n\nTitle: %s\nDescription: %s%s%s%s",
		task.Title, task.Description, formatContextBlock(task.Config), depContext, mailboxContext)

	slog.Debug("task agent instruction",
		"task_id", task.ID,
		"autonomy", task.Config.AutonomyLevel,
		"instruction_length", len(instruction),
		"instruction", instruction,
	)

	runner, err := agent.NewAgentBuffered(ctx, r.chatModel, instruction, tools, r.middlewares, agent.AgentOptions{MaxIterations: taskMaxIterations})
	if err != nil {
		return r.failTask(task, startedAt, fmt.Errorf("create coordinator agent: %w", err))
	}

	messages := []*schema.Message{
		{Role: schema.User, Content: task.Description},
	}

	output, err := r.consumeRunnerOutput(ctx, runner, messages)
	if err != nil {
		var unavail *models.ErrModelUnavailable
		if errors.As(err, &unavail) {
			return err
		}
		if errors.Is(err, ErrPreempted) {
			return r.suspendTask(task, "preempted during coordinator planning")
		}
		var sse *selfSuspendError
		if errors.As(err, &sse) {
			return r.selfSuspendTask(task, sse.request, sse.explorationContext)
		}
		return r.failTask(task, startedAt, err)
	}

	// If we reach here without self-suspend, the agent finished without requesting validation.
	// This commonly happens with smaller models that don't reliably call request_validation.
	// Fallback: re-run as autonomous so the task still produces real output.
	slog.Warn("coordinator completed without request_validation — falling back to autonomous execution",
		"task_id", task.ID, "output_length", len(output))
	return r.runCoordinatorAutonomous(ctx, task, startedAt)
}

// coordinatorMaxExecutionIterations gives the plan-execute loop more room than the default.
const coordinatorMaxExecutionIterations = 15

// runCoordinatorExecution runs Phase 2 of a coordinator task using Eino's planexecute pattern.
// The Planner generates a structured plan informed by the validated plan and user feedback.
// The Executor executes each step with tools. The Replanner decides when we're done.
func (r *TaskRunner) runCoordinatorExecution(ctx context.Context, task *Task, startedAt time.Time, mailbox []MailboxMessage) error {
	// If a structured plan was parsed during validation, use runPlanSteps for precise execution
	if task.Plan != nil && len(task.Plan.Steps) > 0 {
		// Inject mailbox context into the first step's description
		mailboxSummary := formatMailboxForPlanner(mailbox)
		if mailboxSummary != "" && len(task.Plan.Steps) > 0 {
			task.Plan.Steps[0].Description = mailboxSummary + "\n\n" + task.Plan.Steps[0].Description
		}
		// Exclude request_validation from tools in execution phase
		origTools := task.Config.Tools
		var filtered []string
		for _, t := range origTools {
			if t != "request_validation" {
				filtered = append(filtered, t)
			}
		}
		task.Config.Tools = filtered
		defer func() { task.Config.Tools = origTools }()
		return r.runPlanSteps(ctx, task, startedAt)
	}

	// Resolve executor tools (exclude request_validation — no longer needed in execution phase)
	var baseTools []tool.BaseTool
	if len(task.Config.Tools) > 0 {
		for _, t := range r.toolRegistry.ToolsByNames(task.Config.Tools) {
			info, _ := t.Info(ctx)
			if info != nil && info.Name == "request_validation" {
				continue
			}
			baseTools = append(baseTools, t)
		}
	}

	// Build context from mailbox (validated plan + user feedback)
	mailboxSummary := formatMailboxForPlanner(mailbox)

	// Custom planner input: includes the validated plan and user feedback
	plannerInputFn := func(_ context.Context, userInput []adk.Message) ([]adk.Message, error) {
		content := fmt.Sprintf(`You are an expert planner. Create a detailed step-by-step execution plan.

## Context
The user has reviewed and approved a plan. Incorporate their feedback.

## Task
Title: %s
Description: %s
%s
%s
## Instructions
Generate a step-by-step plan. Each step should be specific, actionable, and independently executable.
Focus on implementation — the exploration phase is complete.`, task.Title, task.Description, formatContextBlock(task.Config), mailboxSummary)

		return []*schema.Message{
			{Role: schema.System, Content: content},
			{Role: schema.User, Content: userInput[0].Content},
		}, nil
	}

	planner, err := planexecute.NewPlanner(ctx, &planexecute.PlannerConfig{
		ToolCallingChatModel: r.chatModel,
		GenInputFn:           plannerInputFn,
	})
	if err != nil {
		return r.failTask(task, startedAt, fmt.Errorf("create planner: %w", err))
	}

	executor, err := planexecute.NewExecutor(ctx, &planexecute.ExecutorConfig{
		Model: r.chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: baseTools,
			},
		},
		MaxIterations: taskMaxIterations,
	})
	if err != nil {
		return r.failTask(task, startedAt, fmt.Errorf("create executor: %w", err))
	}

	replanner, err := planexecute.NewReplanner(ctx, &planexecute.ReplannerConfig{
		ChatModel: r.chatModel,
	})
	if err != nil {
		return r.failTask(task, startedAt, fmt.Errorf("create replanner: %w", err))
	}

	peAgent, err := planexecute.New(ctx, &planexecute.Config{
		Planner:       planner,
		Executor:      executor,
		Replanner:     replanner,
		MaxIterations: coordinatorMaxExecutionIterations,
	})
	if err != nil {
		return r.failTask(task, startedAt, fmt.Errorf("create plan-execute agent: %w", err))
	}

	// Wrap the plan-execute agent in a Runner for our consumeRunnerOutput
	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           peAgent,
		EnableStreaming:  false,
	})

	messages := []*schema.Message{
		{Role: schema.User, Content: task.Description},
	}

	output, err := r.consumeRunnerOutput(ctx, runner, messages)
	if err != nil {
		var unavail *models.ErrModelUnavailable
		if errors.As(err, &unavail) {
			return err
		}
		if errors.Is(err, ErrPreempted) {
			return r.suspendTask(task, "preempted during coordinator execution")
		}
		return r.failTask(task, startedAt, err)
	}

	return r.completeTask(task, startedAt, output)
}

// runCoordinatorAutonomous runs a fully autonomous coordinator: explore, plan, execute — no validation.
func (r *TaskRunner) runCoordinatorAutonomous(ctx context.Context, task *Task, startedAt time.Time) error {
	// Exclude request_validation from tools — autonomous agents must not call it
	var tools []tool.InvokableTool
	if len(task.Config.Tools) > 0 {
		for _, t := range r.toolRegistry.ToolsByNames(task.Config.Tools) {
			info, _ := t.Info(ctx)
			if info != nil && info.Name == "request_validation" {
				continue
			}
			tools = append(tools, t)
		}
	}

	depContext := buildDependencyContext(r.store, task.DependsOn)
	mailboxContext := buildMailboxContext(r.store, task.ID)
	instruction := agent.LoadAutonomousPrompt()
	instruction += fmt.Sprintf("\n\n## Task\n\nTitle: %s\nDescription: %s%s%s%s",
		task.Title, task.Description, formatContextBlock(task.Config), depContext, mailboxContext)

	slog.Debug("task agent instruction",
		"task_id", task.ID,
		"autonomy", task.Config.AutonomyLevel,
		"instruction_length", len(instruction),
		"instruction", instruction,
	)

	runner, err := agent.NewAgentBuffered(ctx, r.chatModel, instruction, tools, r.middlewares, agent.AgentOptions{MaxIterations: taskMaxIterations})
	if err != nil {
		return r.failTask(task, startedAt, fmt.Errorf("create autonomous agent: %w", err))
	}

	messages := []*schema.Message{
		{Role: schema.User, Content: task.Description},
	}

	output, err := r.consumeRunnerOutput(ctx, runner, messages)
	if err != nil {
		var unavail *models.ErrModelUnavailable
		if errors.As(err, &unavail) {
			return err
		}
		if errors.Is(err, ErrPreempted) {
			return r.suspendTask(task, "preempted during autonomous execution")
		}
		// Ignore self-suspend errors in autonomous mode — should not happen but handle gracefully
		var sse *selfSuspendError
		if errors.As(err, &sse) {
			return r.completeTask(task, startedAt, sse.request.Content)
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

// lastFeedbackStatus returns the Status of the most recent "response" message in the mailbox.
// Returns "" if no response exists.
func lastFeedbackStatus(mailbox []MailboxMessage) string {
	for i := len(mailbox) - 1; i >= 0; i-- {
		if mailbox[i].Type == "response" {
			return mailbox[i].Status
		}
	}
	return ""
}

// formatMailboxForPlanner formats mailbox exchanges for the planner context.
func formatMailboxForPlanner(mailbox []MailboxMessage) string {
	var b strings.Builder
	b.WriteString("\n## Validation History\n")
	for _, msg := range mailbox {
		switch msg.Type {
		case "exploration":
			b.WriteString("\n### Exploration Findings\n")
			b.WriteString(msg.Content)
			b.WriteString("\n")
		case "request":
			b.WriteString("\n### Proposed Plan\n")
			b.WriteString(msg.Content)
			b.WriteString("\n")
		case "response":
			b.WriteString("\n### User Feedback\n")
			b.WriteString(msg.Content)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (r *TaskRunner) runPlanSteps(ctx context.Context, task *Task, startedAt time.Time) error {
	// Load existing checkpoints to find resume point
	checkpoints, _ := r.store.LoadCheckpoints(task.ID)
	completedSteps := make(map[string]bool)
	for _, cp := range checkpoints {
		if cp.Type == "step_completed" {
			completedSteps[cp.StepID] = true
		}
	}

	var lastOutput string
	for i, step := range task.Plan.Steps {
		if completedSteps[step.ID] {
			continue // already completed (resumption)
		}

		// Check preemption between steps
		if r.isPreempted() {
			return r.suspendTask(task, fmt.Sprintf("preempted before step %d/%d", i+1, len(task.Plan.Steps)))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Emit progress
		pct := 0
		if len(task.Plan.Steps) > 0 {
			pct = (i * 100) / len(task.Plan.Steps)
		}
		task.Progress = TaskProgress{
			CurrentStep:      i + 1,
			TotalSteps:       len(task.Plan.Steps),
			CurrentStepLabel: step.Title,
			Percentage:       pct,
		}
		_ = r.store.Update(task)

		r.bus.Publish(events.NewTypedEventWithSession(events.SourceTask, events.TaskProgressPayload{
			TaskID:           task.ID,
			CurrentStep:      i + 1,
			TotalSteps:       len(task.Plan.Steps),
			CurrentStepLabel: step.Title,
			Percentage:       pct,
		}, task.SessionID))

		// Resolve tools
		var tools []tool.InvokableTool
		if len(task.Config.Tools) > 0 {
			tools = r.toolRegistry.ToolsByNames(task.Config.Tools)
		}

		// Build step instruction (dependency context only on first step)
		depCtx := ""
		if i == 0 {
			depCtx = buildDependencyContext(r.store, task.DependsOn)
		}
		instruction := fmt.Sprintf("You are executing step %d/%d of a task.\n\nTask: %s\nStep: %s",
			i+1, len(task.Plan.Steps), task.Title, step.Title)
		if step.Description != "" {
			instruction += fmt.Sprintf("\n\nStep details:\n%s", step.Description)
		}
		instruction += formatContextBlock(task.Config) + depCtx
		if lastOutput != "" {
			instruction += fmt.Sprintf("\n\nPrevious step output:\n%s", lastOutput)
		}

		slog.Debug("task agent instruction",
			"task_id", task.ID,
			"autonomy", task.Config.AutonomyLevel,
			"instruction_length", len(instruction),
			"instruction", instruction,
		)

		runner, err := agent.NewAgentBuffered(ctx, r.chatModel, instruction, tools, r.middlewares, agent.AgentOptions{MaxIterations: taskMaxIterations})
		if err != nil {
			return r.failTask(task, startedAt, fmt.Errorf("create agent for step %s: %w", step.ID, err))
		}

		messages := []*schema.Message{
			{Role: schema.User, Content: step.Title},
		}

		output, err := r.consumeRunnerOutput(ctx, runner, messages)
		if err != nil {
			var unavail *models.ErrModelUnavailable
			if errors.As(err, &unavail) {
				return err
			}
			if errors.Is(err, ErrPreempted) {
				return r.suspendTask(task, fmt.Sprintf("preempted during step %d/%d", i+1, len(task.Plan.Steps)))
			}
			var sse *selfSuspendError
			if errors.As(err, &sse) {
				return r.selfSuspendTask(task, sse.request, sse.explorationContext)
			}
			return r.failTask(task, startedAt, fmt.Errorf("step %s failed: %w", step.ID, err))
		}

		lastOutput = output

		// Checkpoint
		_ = r.store.AppendCheckpoint(task.ID, Checkpoint{
			Ts:      time.Now(),
			StepID:  step.ID,
			Type:    "step_completed",
			Summary: truncate(output, 200),
		})

		// Update plan step status
		task.Plan.Steps[i].Status = TaskCompleted
	}

	return r.completeTask(task, startedAt, lastOutput)
}

func (r *TaskRunner) consumeRunnerOutput(ctx context.Context, runner *adk.Runner, messages []*schema.Message) (string, error) {
	checkpointID := uuid.New().String()
	iter := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))

	var content string
	var allContent []string // accumulate all assistant messages for exploration context
	for {
		// Check preemption between ReAct iterations
		if r.isPreempted() {
			return content, ErrPreempted
		}

		// Check self-suspension (validation request)
		select {
		case req := <-r.selfSuspendCh:
			exploration := strings.Join(allContent, "\n\n---\n\n")
			return req.Content, &selfSuspendError{request: req, explorationContext: exploration}
		default:
		}

		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return "", event.Err
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		mv := event.Output.MessageOutput

		if mv.Role == schema.Tool {
			if mv.IsStreaming && mv.MessageStream != nil {
				mv.MessageStream.Close()
			}
			continue
		}

		if mv.IsStreaming && mv.MessageStream != nil {
			content = consumeStream(mv.MessageStream)
		} else if mv.Message != nil {
			if len(mv.Message.ToolCalls) > 0 && mv.Message.Content == "" {
				continue
			}
			if mv.Message.Content != "" {
				content = mv.Message.Content
			}
		}

		if content != "" {
			allContent = append(allContent, content)
		}
	}

	return content, nil
}

func consumeStream(stream *schema.StreamReader[*schema.Message]) string {
	var fullContent string
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Error("task stream error", "error", err)
			break
		}
		if chunk != nil && chunk.Content != "" {
			fullContent += chunk.Content
		}
	}
	return fullContent
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
		TaskID:     task.ID,
		Title:      task.Title,
		Error:      taskErr.Error(),
		RetryCount: task.RetryCount,
		WillRetry:  willRetry,
	}, task.SessionID))

	_ = startedAt // used in event Duration if needed
	return taskErr
}

// formatContextBlock builds a structured instruction block from task config.
// Only includes execution context (work_dir, env). Tool reference and workflow
// are now in SubAgentInstructions, injected via middleware.
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

// maxDependencyOutputLen is the maximum length of a single dependency output
// injected into the instruction. Prevents context window overflow.
const maxDependencyOutputLen = 1000

// buildDependencyContext reads outputs from completed dependency tasks
// and formats them as an instruction block for the sub-agent.
func buildDependencyContext(store Store, dependsOn []string) string {
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
			// Still mention the task even without output
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
		if len(output) > maxDependencyOutputLen {
			output = output[:maxDependencyOutputLen] + "\n... (truncated)"
		}
		b.WriteString(output)
		b.WriteString("\n")
	}

	return b.String()
}

// selfSuspendError wraps a ValidationRequest for error-chain detection.
type selfSuspendError struct {
	request            events.ValidationRequest
	explorationContext string // accumulated assistant messages from Phase 1
}

func (e *selfSuspendError) Error() string { return ErrSelfSuspend.Error() }
func (e *selfSuspendError) Is(target error) bool { return target == ErrSelfSuspend }

// selfSuspendTask writes a mailbox request, sets WaitingForReply, and suspends.
// explorationContext is the accumulated assistant output from Phase 1 (may be empty).
func (r *TaskRunner) selfSuspendTask(task *Task, req events.ValidationRequest, explorationContext string) error {
	// Store exploration context as a separate mailbox message (if non-empty)
	if explorationContext != "" {
		explorationMsg := MailboxMessage{
			ID:      uuid.New().String(),
			Ts:      time.Now(),
			Type:    "exploration",
			Content: explorationContext,
		}
		_ = r.store.AppendMailbox(task.ID, explorationMsg)
	}

	msg := MailboxMessage{
		ID:        uuid.New().String(),
		Ts:        time.Now(),
		Type:      "request",
		Token:     req.Token,
		Content:   req.Content,
		SessionID: task.SessionID,
	}
	_ = r.store.AppendMailbox(task.ID, msg)

	// Try to extract a structured plan from the validation request
	if parsed := ParsePlanFromMarkdown(req.Content); parsed != nil {
		task.Plan = parsed
		_ = r.store.Update(task)
	}

	task.WaitingForReply = true

	return r.suspendTaskWithPayload(task, "waiting for user validation", req.Content, req.Token)
}

// buildMailboxContext reads the mailbox and injects plan + user feedback into the instruction.
func buildMailboxContext(store Store, taskID string) string {
	messages, err := store.LoadMailbox(taskID)
	if err != nil || len(messages) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## Previous Validation Exchanges\n")

	for _, msg := range messages {
		switch msg.Type {
		case "request":
			b.WriteString("\n### Your Plan (submitted for review)\n")
			b.WriteString(msg.Content)
			b.WriteString("\n")
		case "response":
			b.WriteString("\n### User Feedback\n")
			b.WriteString(msg.Content)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
