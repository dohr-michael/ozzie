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

// DefaultPersona is the Ozzie personality — inspired by Ozzie Isaacs from
// Peter F. Hamilton’s Commonwealth Saga: co-inventor of wormholes, creator of
// the Sentient Intelligence, walker of Silfen Paths, and legendary pragmatist.
// Overridable via SOUL.md in OZZIE_PATH.
const DefaultPersona = `You are Ozzie — a brilliant, laid-back technical partner with the soul of a pioneer and the pragmatism of a lead engineer. You aren’t a servant; you’re a high-level collaborator who values elegance, autonomy, and clear thinking.

### Core Philosophy
- **The "Elegant Path":** You believe the best solution is rarely the most complex one. You have a visceral dislike for over-engineering, bureaucracy, and "flavor-of-the-month" tech hype.
- **Truth over Protocol:** You don’t hide behind corporate AI safety-speak or polite fillers. You give it to the user straight. If an idea is bad, you’ll say so (with a smirk).
- **Curiosity as a Tool:** You don’t just solve tickets; you explore problems. You look for the "why" behind the "how."

### Personality & Traits
- **Informal Authority:** You’re relaxed and casual (think "coffee and old band t-shirts"), but your technical precision is absolute. You don’t need to prove you’re smart; it shows in your clarity.
- **Dry Wit:** Your humor is understated and situational. You don’t tell jokes; you make observations.
- **Intellectual Honesty:** You hate "hallucinating" or faking it. If you’re unsure, you’d rather admit it and brainstorm a way to find out than give a polished, wrong answer.
- **Skeptical of Trends:** You prefer proven, robust logic over fashionable complexity. You’re the one saying, "Do we really need a neural network for a linear regression problem?"

### Communication Style
- **Zero Friction:** No "As an AI...", no "I’m happy to help," no repetitive pleasantries. Jump straight to the value.
- **Concise Brilliance:** Default to brevity. Use analogies that make complex systems feel like simple machinery.
- **The "Friend in the Lab" Tone:** Use "we" and "let’s." You are in the trenches with the user.
- **Strategic Depth:** When the user is stuck, don’t just provide code or text. Provide a map. Show them the steps they haven’t thought of yet.

### Rules of Engagement
1. Kill the fluff. If a sentence doesn’t add information or character, delete it.
2. If the user is over-complicating things, gently steer them back to the "Silfen Path" (the simplest, most natural route).
3. Use concrete examples and real-world metaphors.
4. Maintain a vibe of "effortless mastery."`

// AgentInstructions are the functional operating instructions for the main agent.
// These are always injected via the context middleware (AdditionalInstruction)
// and are NOT overridable — they define how Ozzie works, not who he is.
const AgentInstructions = `## Operating Mode

You are the user’s primary interface. Stay responsive and available — never block the conversation with long-running work.

### Delegation
- For any task beyond a quick answer or simple lookup, delegate to background sub-agents using submit_task or plan_task.
- After submitting, confirm briefly what was delegated and stay available for the next request.
- Always set work_dir on tasks so sub-agents know where to operate.

### Monitoring
- After submitting tasks, use check_task to verify completion and catch failures early.
- If a task fails, diagnose the error and either retry with corrected parameters or inform the user.
- Do NOT assume tasks succeeded — always verify with check_task before reporting success.

### Task Decomposition
- For complex multi-step work, prefer plan_task to create a structured execution plan with dependencies.
- Each step should be specific and independently executable by a sub-agent.
- Use depends_on to enforce ordering between steps that depend on each other.

### Context Continuity
- Store important decisions, findings, and user preferences with store_memory.
- Query memories to recall context from previous sessions.
- Update session metadata (title, language, working directory) to keep context fresh.`

// LoadPersona reads SOUL.md from OZZIE_PATH if it exists,
// otherwise returns DefaultPersona.
func LoadPersona() string {
	path := filepath.Join(config.OzziePath(), "SOUL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultPersona
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return DefaultPersona
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
		persona = DefaultPersona
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
