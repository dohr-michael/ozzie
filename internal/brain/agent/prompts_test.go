package agent

import (
	"testing"

	"github.com/dohr-michael/ozzie/internal/prompt"
)

func TestPersonaForTier(t *testing.T) {
	t.Run("large returns full", func(t *testing.T) {
		got := PersonaForTier(prompt.DefaultPersona, TierLarge)
		if got != prompt.DefaultPersona {
			t.Errorf("expected DefaultPersona for TierLarge")
		}
	})
	t.Run("medium returns full", func(t *testing.T) {
		got := PersonaForTier(prompt.DefaultPersona, TierMedium)
		if got != prompt.DefaultPersona {
			t.Errorf("expected DefaultPersona for TierMedium")
		}
	})
	t.Run("small returns compact", func(t *testing.T) {
		got := PersonaForTier(prompt.DefaultPersona, TierSmall)
		if got != prompt.DefaultPersonaCompact {
			t.Errorf("expected DefaultPersonaCompact for TierSmall")
		}
	})
	t.Run("small with custom returns custom", func(t *testing.T) {
		custom := "custom persona from SOUL.md"
		got := PersonaForTier(custom, TierSmall)
		if got != custom {
			t.Errorf("expected custom persona to be preserved even for TierSmall")
		}
	})
}

func TestAgentInstructionsForTier(t *testing.T) {
	if AgentInstructionsForTier(TierLarge) != prompt.AgentInstructions {
		t.Error("expected full instructions for TierLarge")
	}
	if AgentInstructionsForTier(TierSmall) != prompt.AgentInstructionsCompact {
		t.Error("expected compact instructions for TierSmall")
	}
}

func TestSubAgentInstructionsForTier(t *testing.T) {
	if SubAgentInstructionsForTier(TierLarge) != prompt.SubAgentInstructions {
		t.Error("expected full instructions for TierLarge")
	}
	if SubAgentInstructionsForTier(TierSmall) != prompt.SubAgentInstructionsCompact {
		t.Error("expected compact instructions for TierSmall")
	}
}
