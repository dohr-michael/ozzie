package agent

import (
	"testing"

	"github.com/dohr-michael/ozzie/internal/brain"
	"github.com/dohr-michael/ozzie/internal/core/prompt"
)

func TestPersonaForTier(t *testing.T) {
	t.Run("large returns full", func(t *testing.T) {
		got := PersonaForTier(prompt.DefaultPersona, brain.TierLarge)
		if got != prompt.DefaultPersona {
			t.Errorf("expected DefaultPersona for brain.TierLarge")
		}
	})
	t.Run("medium returns full", func(t *testing.T) {
		got := PersonaForTier(prompt.DefaultPersona, brain.TierMedium)
		if got != prompt.DefaultPersona {
			t.Errorf("expected DefaultPersona for brain.TierMedium")
		}
	})
	t.Run("small returns compact", func(t *testing.T) {
		got := PersonaForTier(prompt.DefaultPersona, brain.TierSmall)
		if got != prompt.DefaultPersonaCompact {
			t.Errorf("expected DefaultPersonaCompact for brain.TierSmall")
		}
	})
	t.Run("small with custom returns custom", func(t *testing.T) {
		custom := "custom persona from SOUL.md"
		got := PersonaForTier(custom, brain.TierSmall)
		if got != custom {
			t.Errorf("expected custom persona to be preserved even for brain.TierSmall")
		}
	})
}

func TestAgentInstructionsForTier(t *testing.T) {
	if AgentInstructionsForTier(brain.TierLarge) != prompt.AgentInstructions {
		t.Error("expected full instructions for brain.TierLarge")
	}
	if AgentInstructionsForTier(brain.TierSmall) != prompt.AgentInstructionsCompact {
		t.Error("expected compact instructions for brain.TierSmall")
	}
}

func TestSubAgentInstructionsForTier(t *testing.T) {
	if SubAgentInstructionsForTier(brain.TierLarge) != prompt.SubAgentInstructions {
		t.Error("expected full instructions for brain.TierLarge")
	}
	if SubAgentInstructionsForTier(brain.TierSmall) != prompt.SubAgentInstructionsCompact {
		t.Error("expected compact instructions for brain.TierSmall")
	}
}
