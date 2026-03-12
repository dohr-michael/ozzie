package agent

import (
	"github.com/cloudwego/eino/adk"

	"github.com/dohr-michael/ozzie/internal/core/brain"
)

// NewSubAgentMiddleware returns an AgentMiddleware that injects SubAgentInstructions
// (and optionally the runtime instruction) into every sub-agent via AdditionalInstruction.
// This is the sub-agent equivalent of the context middleware that injects AgentInstructions
// for the main agent.
func NewSubAgentMiddleware(runtimeInstruction string, tier brain.ModelTier) adk.AgentMiddleware {
	instruction := SubAgentInstructionsForTier(tier)
	if runtimeInstruction != "" {
		instruction += "\n\n" + runtimeInstruction
	}
	return adk.AgentMiddleware{
		AdditionalInstruction: instruction,
	}
}
