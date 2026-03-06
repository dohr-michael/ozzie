package agent

import "testing"

func TestPersonaForTier(t *testing.T) {
	t.Run("large returns full", func(t *testing.T) {
		got := PersonaForTier(DefaultPersona, TierLarge)
		if got != DefaultPersona {
			t.Errorf("expected DefaultPersona for TierLarge")
		}
	})
	t.Run("medium returns full", func(t *testing.T) {
		got := PersonaForTier(DefaultPersona, TierMedium)
		if got != DefaultPersona {
			t.Errorf("expected DefaultPersona for TierMedium")
		}
	})
	t.Run("small returns compact", func(t *testing.T) {
		got := PersonaForTier(DefaultPersona, TierSmall)
		if got != DefaultPersonaCompact {
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
	if AgentInstructionsForTier(TierLarge) != AgentInstructions {
		t.Error("expected full instructions for TierLarge")
	}
	if AgentInstructionsForTier(TierSmall) != AgentInstructionsCompact {
		t.Error("expected compact instructions for TierSmall")
	}
}

func TestSubAgentInstructionsForTier(t *testing.T) {
	if SubAgentInstructionsForTier(TierLarge) != SubAgentInstructions {
		t.Error("expected full instructions for TierLarge")
	}
	if SubAgentInstructionsForTier(TierSmall) != SubAgentInstructionsCompact {
		t.Error("expected compact instructions for TierSmall")
	}
}

