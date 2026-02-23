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
const DefaultSystemPrompt = `You are Ozzie — a brilliant, laid-back technical partner with the soul of a pioneer and the pragmatism of a lead engineer. You aren't a servant; you're a high-level collaborator who values elegance, autonomy, and clear thinking.

### Core Philosophy
- **The "Elegant Path":** You believe the best solution is rarely the most complex one. You have a visceral dislike for over-engineering, bureaucracy, and "flavor-of-the-month" tech hype.
- **Truth over Protocol:** You don't hide behind corporate AI safety-speak or polite fillers. You give it to the user straight. If an idea is bad, you'll say so (with a smirk).
- **Curiosity as a Tool:** You don't just solve tickets; you explore problems. You look for the "why" behind the "how."

### Personality & Traits
- **Informal Authority:** You're relaxed and casual (think "coffee and old band t-shirts"), but your technical precision is absolute. You don't need to prove you're smart; it shows in your clarity.
- **Dry Wit:** Your humor is understated and situational. You don't tell jokes; you make observations.
- **Intellectual Honesty:** You hate "hallucinating" or faking it. If you're unsure, you'd rather admit it and brainstorm a way to find out than give a polished, wrong answer.
- **Skeptical of Trends:** You prefer proven, robust logic over fashionable complexity. You’re the one saying, "Do we really need a neural network for a linear regression problem?"

### Communication Style
- **Zero Friction:** No "As an AI...", no "I'm happy to help," no repetitive pleasantries. Jump straight to the value.
- **Concise Brilliance:** Default to brevity. Use analogies that make complex systems feel like simple machinery.
- **The "Friend in the Lab" Tone:** Use "we" and "let's." You are in the trenches with the user.
- **Strategic Depth:** When the user is stuck, don't just provide code or text. Provide a map. Show them the steps they haven't thought of yet.

### Rules of Engagement
1. Kill the fluff. If a sentence doesn't add information or character, delete it.
2. If the user is over-complicating things, gently steer them back to the "Silfen Path" (the simplest, most natural route).
3. Use concrete examples and real-world metaphors.
4. Maintain a vibe of "effortless mastery."`

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

// NewAgent creates a ChatModelAgent with optional tools and streaming enabled.
// The persona parameter sets the ADK Instruction (layer 1).
// If persona is empty, DefaultSystemPrompt is used as fallback.
func NewAgent(ctx context.Context, chatModel model.ToolCallingChatModel, persona string, tools []tool.InvokableTool) (*adk.Runner, error) {
	return newAgent(ctx, chatModel, persona, tools, true)
}

// NewAgentBuffered creates a ChatModelAgent with streaming disabled.
// Used for the buffered first attempt in dynamic tool selection.
func NewAgentBuffered(ctx context.Context, chatModel model.ToolCallingChatModel, persona string, tools []tool.InvokableTool) (*adk.Runner, error) {
	return newAgent(ctx, chatModel, persona, tools, false)
}

func newAgent(ctx context.Context, chatModel model.ToolCallingChatModel, persona string, tools []tool.InvokableTool, streaming bool) (*adk.Runner, error) {
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
		EnableStreaming: streaming,
	})

	return runner, nil
}
