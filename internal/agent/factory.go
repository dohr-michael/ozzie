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
	chatModel model.ToolCallingChatModel
	persona   string
}

// NewAgentFactory creates a new AgentFactory.
func NewAgentFactory(chatModel model.ToolCallingChatModel, persona string) *AgentFactory {
	return &AgentFactory{
		chatModel: chatModel,
		persona:   persona,
	}
}

// CreateRunner creates a new streaming ADK Runner configured with the given tools.
func (f *AgentFactory) CreateRunner(ctx context.Context, tools []tool.InvokableTool) (*adk.Runner, error) {
	return NewAgent(ctx, f.chatModel, f.persona, tools)
}

// Persona returns the persona string used for agent creation.
func (f *AgentFactory) Persona() string { return f.persona }

// CreateRunnerBuffered creates a new non-streaming ADK Runner.
// Used for the buffered first attempt in dynamic tool selection to avoid
// HTTP/2 stream errors when the response isn't consumed as a stream.
func (f *AgentFactory) CreateRunnerBuffered(ctx context.Context, tools []tool.InvokableTool) (*adk.Runner, error) {
	return NewAgentBuffered(ctx, f.chatModel, f.persona, tools)
}
