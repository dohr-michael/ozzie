package agent

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
)

// AgentFactory creates a fresh ADK Runner per call.
// This is necessary because Eino's runner freezes its tool set after the first
// Run() call (sync.Once + atomic.frozen), so dynamic tool selection requires a
// new runner for each turn.
type AgentFactory struct {
	chatModel   model.ToolCallingChatModel
	persona     string
	middlewares []adk.AgentMiddleware // base middlewares (without recovery)
}

// NewAgentFactory creates a new AgentFactory.
func NewAgentFactory(chatModel model.ToolCallingChatModel, persona string, middlewares []adk.AgentMiddleware) *AgentFactory {
	return &AgentFactory{
		chatModel:   chatModel,
		persona:     persona,
		middlewares: middlewares,
	}
}

// buildMiddlewares returns a fresh middleware chain with a new tool recovery
// middleware prepended. Each runner gets its own recovery instance so that
// error counters are isolated per turn/session.
func (f *AgentFactory) buildMiddlewares() []adk.AgentMiddleware {
	recoveryMw := adk.AgentMiddleware{
		WrapToolCall: NewToolRecoveryMiddleware(ToolRecoveryConfig{}),
	}
	mws := make([]adk.AgentMiddleware, 0, 1+len(f.middlewares))
	mws = append(mws, recoveryMw)
	mws = append(mws, f.middlewares...)
	return mws
}

// CreateRunner creates a new streaming ADK Runner configured with the given tools.
func (f *AgentFactory) CreateRunner(ctx context.Context, tools []tool.InvokableTool) (*adk.Runner, error) {
	return NewAgent(ctx, f.chatModel, f.persona, tools, f.buildMiddlewares())
}

// Persona returns the persona string used for agent creation.
func (f *AgentFactory) Persona() string { return f.persona }

// CreateRunnerBuffered creates a new non-streaming ADK Runner.
// Used for the buffered first attempt in dynamic tool selection to avoid
// HTTP/2 stream errors when the response isn't consumed as a stream.
func (f *AgentFactory) CreateRunnerBuffered(ctx context.Context, tools []tool.InvokableTool) (*adk.Runner, error) {
	return NewAgentBuffered(ctx, f.chatModel, f.persona, tools, f.buildMiddlewares())
}
