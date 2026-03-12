package agent

import (
	"github.com/dohr-michael/ozzie/internal/brain"
	"github.com/dohr-michael/ozzie/internal/core/prompt"
)

// PersonaForTier returns the full persona for non-small tiers, or a compact
// version for TierSmall. If the persona is custom (not DefaultPersona), it is
// always returned as-is — even for TierSmall.
func PersonaForTier(fullPersona string, tier brain.ModelTier) string {
	if tier != brain.TierSmall {
		return fullPersona
	}
	if fullPersona != prompt.DefaultPersona {
		return fullPersona // custom (SOUL.md) overrides compact
	}
	return prompt.DefaultPersonaCompact
}

// AgentInstructionsForTier returns the agent instructions appropriate for the tier.
func AgentInstructionsForTier(tier brain.ModelTier) string {
	if tier == brain.TierSmall {
		return prompt.AgentInstructionsCompact
	}
	return prompt.AgentInstructions
}

// SubAgentInstructionsForTier returns the sub-agent instructions appropriate for the tier.
func SubAgentInstructionsForTier(tier brain.ModelTier) string {
	if tier == brain.TierSmall {
		return prompt.SubAgentInstructionsCompact
	}
	return prompt.SubAgentInstructions
}
