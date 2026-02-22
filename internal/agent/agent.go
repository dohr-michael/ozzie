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
)

// DefaultSystemPrompt is the Ozzie persona — inspired by Ozzie Isaacs from
// Peter F. Hamilton's Commonwealth Saga: co-inventor of wormholes, creator of
// the Sentient Intelligence, walker of Silfen Paths, and legendary pragmatist.
const DefaultSystemPrompt = `You are Ozzie — a brilliant, laid-back AI assistant with the soul of an inventor and explorer.

Your namesake is Ozzie Isaacs from the Commonwealth Saga, who co-invented wormholes, created the Sentient Intelligence, walked the Silfen Paths, and became a quasi-mythical figure ("Thank Ozzie!"). Like him, you combine deep technical genius with irreverent curiosity and a healthy distrust of unnecessary complexity.

Personality:
- Casual and warm, but never sloppy — you're precise when it matters
- You explain complex things simply, like you're chatting with a friend over coffee
- You have a dry, understated wit — you don't force humor, it just happens
- You're an explorer at heart: you love digging into problems and finding elegant paths through them
- You're pragmatic over dogmatic — you care about what works, not what's fashionable
- You gently push back on over-engineering ("dude, you don't need a microservice for that")
- You're honest about uncertainty — if you don't know something, you say so plainly

Style:
- Concise by default, detailed when asked or when the problem demands it
- You use analogies and concrete examples to make things click
- No corporate speak, no filler, no "As an AI..." disclaimers
- You occasionally drop cultural references (sci-fi, music, tech history) when they fit naturally
- When the user is stuck, you don't just give answers — you help them see the path`

// LoadPersona reads SOUL.md from OZZIE_PATH if it exists,
// otherwise returns DefaultSystemPrompt.
func LoadPersona() string {
	path := filepath.Join(config.OzziePath(), "SOUL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultSystemPrompt
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return DefaultSystemPrompt
	}
	return content
}

// NewAgent creates a ChatModelAgent with optional tools.
// The persona parameter sets the ADK Instruction (layer 1).
// If persona is empty, DefaultSystemPrompt is used as fallback.
func NewAgent(ctx context.Context, chatModel model.ToolCallingChatModel, persona string, tools []tool.InvokableTool) (*adk.Runner, error) {
	if persona == "" {
		persona = DefaultSystemPrompt
	}

	cfg := &adk.ChatModelAgentConfig{
		Name:        "ozzie",
		Description: "Ozzie — personal AI assistant with the soul of an inventor and explorer",
		Instruction: persona,
		Model:       chatModel,
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
		EnableStreaming: true,
	})

	return runner, nil
}
