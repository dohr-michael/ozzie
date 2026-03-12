// Package agent provides the Ozzie agent bridge using Eino ADK.
package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/prompt"
)

// LoadPersona reads SOUL.md from OZZIE_PATH if it exists,
// otherwise returns DefaultPersona.
func LoadPersona() string {
	path := filepath.Join(config.OzziePath(), "SOUL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return prompt.DefaultPersona
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return prompt.DefaultPersona
	}
	return content
}

// AgentOptions configures optional agent behavior.
type AgentOptions struct {
	MaxIterations int // 0 = ADK default
}

// NewAgent creates a ChatModelAgent with optional tools, middlewares, and streaming enabled.
// The persona parameter sets the ADK Instruction (layer 1).
// If persona is empty, DefaultPersona is used as fallback.
func NewAgent(ctx context.Context, chatModel model.ToolCallingChatModel, persona string, tools []tool.InvokableTool, middlewares []adk.AgentMiddleware, opts ...AgentOptions) (*adk.Runner, error) {
	return newAgent(ctx, chatModel, persona, tools, middlewares, true, opts...)
}

// NewAgentBuffered creates a ChatModelAgent with streaming disabled.
// Used for the buffered first attempt in dynamic tool selection.
func NewAgentBuffered(ctx context.Context, chatModel model.ToolCallingChatModel, persona string, tools []tool.InvokableTool, middlewares []adk.AgentMiddleware, opts ...AgentOptions) (*adk.Runner, error) {
	return newAgent(ctx, chatModel, persona, tools, middlewares, false, opts...)
}

func newAgent(ctx context.Context, chatModel model.ToolCallingChatModel, persona string, tools []tool.InvokableTool, middlewares []adk.AgentMiddleware, streaming bool, opts ...AgentOptions) (*adk.Runner, error) {
	if persona == "" {
		persona = prompt.DefaultPersona
	}

	var opt AgentOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	cfg := &adk.ChatModelAgentConfig{
		Name:          "ozzie",
		Description:   "Ozzie — personal AI assistant with the soul of an inventor and explorer",
		Instruction:   persona,
		Model:         chatModel,
		MaxIterations: opt.MaxIterations,
		Middlewares:   middlewares,
	}

	// Register tools with the agent (enables ReAct loop in ADK)
	if len(tools) > 0 {
		baseTools := make([]tool.BaseTool, len(tools))
		for i, t := range tools {
			baseTools[i] = t
		}
		cfg.ToolsConfig.Tools = baseTools
	}

	agent, err := adk.NewChatModelAgent(ctx, cfg)
	if err != nil {
		return nil, err
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: streaming,
	})

	return runner, nil
}
