package skills

import (
	"context"
	"fmt"
	"io"
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
	EventBus      *events.Bus
}

// WorkflowRunner executes a workflow skill by running its DAG of steps.
type WorkflowRunner struct {
	skill *Skill
	dag   *DAG
	cfg   RunnerConfig
}

// NewWorkflowRunner creates a runner for a workflow skill.
func NewWorkflowRunner(skill *Skill, cfg RunnerConfig) (*WorkflowRunner, error) {
	dag, err := NewDAG(skill.Steps)
	if err != nil {
		return nil, fmt.Errorf("build DAG for skill %q: %w", skill.Name, err)
	}

	return &WorkflowRunner{
		skill: skill,
		dag:   dag,
		cfg:   cfg,
	}, nil
}

// Run executes the workflow, running steps in DAG order with parallel execution
// where dependencies allow. Returns the output of the last step in topological order.
func (wr *WorkflowRunner) Run(ctx context.Context, vars map[string]string) (string, error) {
	if err := wr.validateVars(vars); err != nil {
		return "", err
	}

	// Apply defaults
	for name, v := range wr.skill.Vars {
		if _, ok := vars[name]; !ok && v.Default != "" {
			vars[name] = v.Default
		}
	}

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
	for name, v := range wr.skill.Vars {
		if v.Required {
			if _, ok := vars[name]; !ok {
				return fmt.Errorf("skill %q: required variable %q not provided", wr.skill.Name, name)
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
		modelName = wr.skill.Model
	}

	// Emit step started
	wr.cfg.EventBus.Publish(events.NewTypedEventWithSession(events.SourceSkill, events.SkillStepStartedPayload{
		SkillName: wr.skill.Name,
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
	runner, err := agent.NewAgent(ctx, chatModel, instruction, stepTools)
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
	if step.Acceptance != "" {
		sb.WriteString("\n\n## Acceptance Criteria\n\n")
		sb.WriteString(step.Acceptance)
	}

	return sb.String()
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
		SkillName: wr.skill.Name,
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
	var content string

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return content, event.Err
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		mv := event.Output.MessageOutput

		// Skip tool messages
		if mv.Role == schema.Tool {
			if mv.IsStreaming && mv.MessageStream != nil {
				mv.MessageStream.Close()
			}
			continue
		}

		// Collect assistant content
		if mv.IsStreaming && mv.MessageStream != nil {
			for {
				chunk, err := mv.MessageStream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					return content, err
				}
				if chunk != nil && chunk.Content != "" {
					content += chunk.Content
				}
			}
		} else if mv.Message != nil && mv.Message.Content != "" {
			content = mv.Message.Content
		}
	}

	return content, nil
}
