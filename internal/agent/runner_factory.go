package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/brain"
	"github.com/dohr-michael/ozzie/internal/models"
)

// EinoRunnerFactory implements brain.RunnerFactory using the Eino ADK.
type EinoRunnerFactory struct {
	models *models.Registry
}

// NewRunnerFactory creates a RunnerFactory backed by the model registry.
func NewRunnerFactory(models *models.Registry) brain.RunnerFactory {
	return &EinoRunnerFactory{models: models}
}

// CreateRunner creates an ephemeral agent runner for the given model, instruction, and tools.
// An empty model name resolves to the default model.
func (f *EinoRunnerFactory) CreateRunner(ctx context.Context, modelName string, instruction string, tools []brain.Tool, opts ...brain.RunnerOption) (brain.Runner, error) {
	o := brain.ApplyRunnerOpts(opts)

	var chatModel model.ToolCallingChatModel
	var err error

	if modelName == "" {
		chatModel, err = f.models.Default(ctx)
	} else {
		chatModel, err = f.models.Get(ctx, modelName)
		if err != nil {
			// Check for model unavailable before fallback
			var unavail *models.ErrModelUnavailable
			if errors.As(err, &unavail) {
				return nil, &brain.ErrModelUnavailable{Provider: unavail.Provider, Cause: unavail.Cause}
			}
			// Fallback to default
			chatModel, err = f.models.Default(ctx)
		}
	}
	if err != nil {
		var unavail *models.ErrModelUnavailable
		if errors.As(err, &unavail) {
			return nil, &brain.ErrModelUnavailable{Provider: unavail.Provider, Cause: unavail.Cause}
		}
		return nil, fmt.Errorf("resolve model %q: %w", modelName, err)
	}

	einoTools := ConvertToolsToEino(tools)

	// Build middlewares: always prepend tool recovery, then add opaque ones
	var middlewares []adk.AgentMiddleware
	middlewares = append(middlewares, adk.AgentMiddleware{
		WrapToolCall: NewToolRecoveryMiddleware(ToolRecoveryConfig{}),
	})
	for _, m := range o.Middlewares {
		if mw, ok := m.(adk.AgentMiddleware); ok {
			middlewares = append(middlewares, mw)
		}
	}

	var agentOpts AgentOptions
	if o.MaxIterations > 0 {
		agentOpts.MaxIterations = o.MaxIterations
	}

	runner, err := NewAgentBuffered(ctx, chatModel, instruction, einoTools, middlewares, agentOpts)
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}

	return &einoRunner{runner: runner, preemptionCheck: o.PreemptionCheck}, nil
}

// einoRunner wraps an adk.Runner into a brain.Runner.
type einoRunner struct {
	runner          *adk.Runner
	preemptionCheck func() bool
}

// Run executes the agent and returns the concatenated text output.
func (r *einoRunner) Run(ctx context.Context, messages []brain.Message) (string, error) {
	// Convert domain messages to Eino messages
	einoMsgs := make([]*schema.Message, len(messages))
	for i, m := range messages {
		einoMsgs[i] = &schema.Message{
			Role:    schema.RoleType(m.Role),
			Content: m.Content,
		}
	}

	checkpointID := uuid.New().String()
	iter := r.runner.Run(ctx, einoMsgs, adk.WithCheckPointID(checkpointID))

	content, err := ConsumeIterator(iter, IterCallbacks{
		ShouldPreempt: r.preemptionCheck,
	})
	if errors.Is(err, ErrIterPreempted) {
		return content, brain.ErrRunnerPreempted
	}
	// Convert model unavailable errors
	if err != nil {
		var unavail *models.ErrModelUnavailable
		if errors.As(err, &unavail) {
			return content, &brain.ErrModelUnavailable{Provider: unavail.Provider, Cause: unavail.Cause}
		}
	}
	return content, err
}

var _ brain.RunnerFactory = (*EinoRunnerFactory)(nil)
var _ brain.Runner = (*einoRunner)(nil)
