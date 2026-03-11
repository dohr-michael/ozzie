package agent

import "github.com/dohr-michael/ozzie/internal/prompt"

// PersonaForTier returns the full persona for non-small tiers, or a compact
// version for TierSmall. If the persona is custom (not DefaultPersona), it is
// always returned as-is — even for TierSmall.
func PersonaForTier(fullPersona string, tier ModelTier) string {
	if tier != TierSmall {
		return fullPersona
	}
	if fullPersona != prompt.DefaultPersona {
		return fullPersona // custom (SOUL.md) overrides compact
	}
	return prompt.DefaultPersonaCompact
}

// AgentInstructionsForTier returns the agent instructions appropriate for the tier.
func AgentInstructionsForTier(tier ModelTier) string {
	if tier == TierSmall {
		return prompt.AgentInstructionsCompact
	}
	return prompt.AgentInstructions
}

// SubAgentInstructionsForTier returns the sub-agent instructions appropriate for the tier.
func SubAgentInstructionsForTier(tier ModelTier) string {
	if tier == TierSmall {
		return prompt.SubAgentInstructionsCompact
	}
	return prompt.SubAgentInstructions
}
