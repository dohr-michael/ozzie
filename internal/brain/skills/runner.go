package skills

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/agent"
	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/models"
	"github.com/dohr-michael/ozzie/internal/plugins"
)

// RunnerConfig holds dependencies for running skills.
type RunnerConfig struct {
	ModelRegistry *models.Registry
	ToolRegistry  *plugins.ToolRegistry
	EventBus      events.EventBus
	Verifier      *Verifier
}

// WorkflowRunner executes a workflow skill by running its DAG of steps.
type WorkflowRunner struct {
	skillName string
	model     string
	vars      map[string]VarDef
	dag       *DAG
	cfg       RunnerConfig
}

// NewWorkflowRunnerFromDef creates a runner from a SkillMD with a workflow definition.
func NewWorkflowRunnerFromDef(skill *SkillMD, cfg RunnerConfig) (*WorkflowRunner, error) {
	if !skill.HasWorkflow() {
		return nil, fmt.Errorf("skill %q has no workflow definition", skill.Name)
	}

	// Convert StepDef → Step for the DAG
	steps := make([]Step, len(skill.Workflow.Steps))
	for i, sd := range skill.Workflow.Steps {
		steps[i] = sd.ToStep()
	}

	dag, err := NewDAG(steps)
	if err != nil {
		return nil, fmt.Errorf("build DAG for skill %q: %w", skill.Name, err)
	}

	return &WorkflowRunner{
		skillName: skill.Name,
		model:     skill.Workflow.Model,
		vars:      skill.Workflow.Vars,
		dag:       dag,
		cfg:       cfg,
	}, nil
}

// Run executes the workflow, running steps in DAG order with parallel execution
// where dependencies allow. Returns the output of the last step in topological order.
func (wr *WorkflowRunner) Run(ctx context.Context, vars map[string]string) (string, error) {
	if err := wr.validateVars(vars); err != nil {
		return "", err
	}

	// Apply defaults
	for name, v := range wr.vars {
		if _, ok := vars[name]; !ok && v.Default != "" {
			vars[name] = v.Default
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	completed := make(map[string]bool)
	results := make(map[string]string)
	var mu sync.Mutex

	for {
		mu.Lock()
		ready := wr.dag.ReadySteps(completed)
		mu.Unlock()

		if len(ready) == 0 {
			break
		}

		// Run ready steps in parallel
		var wg sync.WaitGroup
		errCh := make(chan error, len(ready))

		for _, stepID := range ready {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()

				mu.Lock()
				resultsCopy := make(map[string]string, len(results))
				for k, v := range results {
					resultsCopy[k] = v
				}
				mu.Unlock()

				output, err := wr.runStep(ctx, id, vars, resultsCopy)
				if err != nil {
					cancel() // cancel sibling steps on first error
					errCh <- fmt.Errorf("step %q: %w", id, err)
					return
				}

				mu.Lock()
				completed[id] = true
				results[id] = output
				mu.Unlock()
			}(stepID)
		}

		wg.Wait()
		close(errCh)

		// Fail-fast: return first error
		if err := <-errCh; err != nil {
			return "", err
		}
	}

	// Return the output of the last step in topological order
	order := wr.dag.TopologicalOrder()
	if len(order) == 0 {
		return "", nil
	}
	return results[order[len(order)-1]], nil
}

func (wr *WorkflowRunner) validateVars(vars map[string]string) error {
	for name, v := range wr.vars {
		if v.Required {
			if _, ok := vars[name]; !ok {
				return fmt.Errorf("skill %q: required variable %q not provided", wr.skillName, name)
			}
		}
	}
	return nil
}

func (wr *WorkflowRunner) runStep(ctx context.Context, stepID string, vars map[string]string, prevResults map[string]string) (string, error) {
	step := wr.dag.Step(stepID)
	if step == nil {
		return "", fmt.Errorf("step %q not found in DAG", stepID)
	}

	sessionID := events.SessionIDFromContext(ctx)
	modelName := step.Model
	if modelName == "" {
		modelName = wr.model
	}

	// Emit step started
	wr.cfg.EventBus.Publish(events.NewTypedEventWithSession(events.SourceSkill, events.SkillStepStartedPayload{
		SkillName: wr.skillName,
		StepID:    stepID,
		StepTitle: step.Title,
		Model:     modelName,
	}, sessionID))

	start := time.Now()

	// Resolve model
	chatModel, err := wr.cfg.ModelRegistry.Get(ctx, modelName)
	if err != nil {
		// Fallback to default model
		chatModel, err = wr.cfg.ModelRegistry.Default(ctx)
		if err != nil {
			wr.emitStepCompleted(sessionID, stepID, step.Title, "", err, start)
			return "", fmt.Errorf("resolve model for step %q: %w", stepID, err)
		}
	}

	// Resolve tools for this step
	stepTools := wr.resolveTools(step.Tools)

	// Build instruction with injected previous results
	instruction := wr.buildStepInstruction(step, vars, prevResults)

	// Create ephemeral agent using agent.NewAgent
	runner, err := agent.NewAgent(ctx, chatModel, instruction, stepTools, nil)
	if err != nil {
		wr.emitStepCompleted(sessionID, stepID, step.Title, "", err, start)
		return "", fmt.Errorf("create agent for step %q: %w", stepID, err)
	}

	// Run the agent
	messages := []*schema.Message{
		{Role: schema.User, Content: "Execute this step."},
	}

	checkpointID := uuid.New().String()
	iter := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))
	output, err := consumeRunnerOutput(iter)

	// Verify-and-retry loop
	if err == nil && step.Acceptance.HasCriteria() && wr.cfg.Verifier != nil {
		output, err = wr.verifyAndRetry(ctx, step, output, vars, prevResults)
	}

	wr.emitStepCompleted(sessionID, stepID, step.Title, output, err, start)
	return output, err
}

// buildStepInstruction builds the full instruction for a step, injecting
// variables and previous step results into the step's base instruction.
func (wr *WorkflowRunner) buildStepInstruction(step *Step, vars map[string]string, prevResults map[string]string) string {
	var sb strings.Builder

	sb.WriteString(step.Instruction)

	// Inject variables
	if len(vars) > 0 {
		sb.WriteString("\n\n## Variables\n\n")
		for name, value := range vars {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", name, value))
		}
	}

	// Inject previous step results
	if len(step.Needs) > 0 {
		sb.WriteString("\n\n## Previous Step Results\n\n")
		for _, need := range step.Needs {
			if result, ok := prevResults[need]; ok {
				sb.WriteString(fmt.Sprintf("### Step: %s\n\n%s\n\n", need, result))
			}
		}
	}

	// Acceptance criteria
	if step.Acceptance.HasCriteria() {
		sb.WriteString("\n\n## Acceptance Criteria\n\n")
		for _, c := range step.Acceptance.Criteria {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
	}

	return sb.String()
}

// verifyAndRetry runs the verify-and-retry loop for a step with acceptance criteria.
func (wr *WorkflowRunner) verifyAndRetry(ctx context.Context, step *Step, output string, vars map[string]string, prevResults map[string]string) (string, error) {
	sessionID := events.SessionIDFromContext(ctx)
	maxAttempts := step.Acceptance.EffectiveMaxAttempts()
	currentOutput := output

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := wr.cfg.Verifier.Verify(ctx, step.Acceptance, step.Title, currentOutput)
		if err != nil {
			slog.Warn("verification failed, treating as pass", "step", step.ID, "error", err)
			return currentOutput, nil
		}

		// Emit verification event
		wr.cfg.EventBus.Publish(events.NewTypedEventWithSession(events.SourceSkill, events.TaskVerificationPayload{
			SkillName: wr.skillName,
			StepID:    step.ID,
			Pass:      result.Pass,
			Score:     result.Score,
			Issues:    result.Issues,
			Attempt:   attempt,
		}, sessionID))

		if result.Pass {
			slog.Info("step verification passed", "step", step.ID, "score", result.Score, "attempt", attempt)
			return currentOutput, nil
		}

		slog.Info("step verification failed, retrying", "step", step.ID, "score", result.Score, "attempt", attempt, "issues", result.Issues)

		// Retry with feedback
		if attempt < maxAttempts {
			retryOutput, retryErr := wr.retryStep(ctx, step, currentOutput, result.Feedback, vars, prevResults)
			if retryErr != nil {
				slog.Warn("retry failed, using last output", "step", step.ID, "error", retryErr)
				return currentOutput, nil
			}
			currentOutput = retryOutput
		}
	}

	// All attempts exhausted — return last output
	slog.Warn("max verification attempts reached", "step", step.ID, "max", maxAttempts)
	return currentOutput, nil
}

// retryStep re-runs a step with augmented instruction including previous output and feedback.
func (wr *WorkflowRunner) retryStep(ctx context.Context, step *Step, previousOutput, feedback string, vars map[string]string, prevResults map[string]string) (string, error) {
	modelName := step.Model
	if modelName == "" {
		modelName = wr.model
	}

	chatModel, err := wr.cfg.ModelRegistry.Get(ctx, modelName)
	if err != nil {
		chatModel, err = wr.cfg.ModelRegistry.Default(ctx)
		if err != nil {
			return "", fmt.Errorf("resolve model for retry: %w", err)
		}
	}

	stepTools := wr.resolveTools(step.Tools)

	// Build augmented instruction
	instruction := wr.buildStepInstruction(step, vars, prevResults)
	instruction += fmt.Sprintf("\n\n## Previous Attempt Output\n\n%s", previousOutput)
	instruction += fmt.Sprintf("\n\n## Verification Feedback\n\nThe previous output did not pass verification. Issues:\n%s\n\nPlease fix these issues and produce an improved output.", feedback)

	runner, err := agent.NewAgent(ctx, chatModel, instruction, stepTools, nil)
	if err != nil {
		return "", fmt.Errorf("create retry agent: %w", err)
	}

	messages := []*schema.Message{
		{Role: schema.User, Content: "Re-execute this step, addressing the verification feedback."},
	}

	checkpointID := uuid.New().String()
	iter := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))
	return consumeRunnerOutput(iter)
}

func (wr *WorkflowRunner) resolveTools(toolNames []string) []tool.InvokableTool {
	if len(toolNames) == 0 {
		return nil
	}

	var result []tool.InvokableTool
	for _, name := range toolNames {
		if t := wr.cfg.ToolRegistry.Tool(name); t != nil {
			result = append(result, t)
		} else {
			slog.Warn("tool not found for step", "tool", name)
		}
	}
	return result
}

func (wr *WorkflowRunner) emitStepCompleted(sessionID, stepID, stepTitle, output string, err error, start time.Time) {
	payload := events.SkillStepCompletedPayload{
		SkillName: wr.skillName,
		StepID:    stepID,
		StepTitle: stepTitle,
		Output:    output,
		Duration:  time.Since(start),
	}
	if err != nil {
		payload.Error = err.Error()
	}
	wr.cfg.EventBus.Publish(events.NewTypedEventWithSession(events.SourceSkill, payload, sessionID))
}

// consumeRunnerOutput drains an ADK AsyncIterator and returns the concatenated text output.
func consumeRunnerOutput(iter *adk.AsyncIterator[*adk.AgentEvent]) (string, error) {
	return agent.ConsumeIterator(iter, agent.IterCallbacks{})
}
