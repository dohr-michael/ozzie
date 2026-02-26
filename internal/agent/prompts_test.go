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

func TestCoordinatorPromptForTier(t *testing.T) {
	t.Run("large returns full", func(t *testing.T) {
		got := CoordinatorPromptForTier(CoordinatorSystemPrompt, TierLarge)
		if got != CoordinatorSystemPrompt {
			t.Error("expected full prompt for TierLarge")
		}
	})
	t.Run("small returns compact", func(t *testing.T) {
		got := CoordinatorPromptForTier(CoordinatorSystemPrompt, TierSmall)
		if got != CoordinatorSystemPromptCompact {
			t.Error("expected compact prompt for TierSmall")
		}
	})
	t.Run("small with custom returns custom", func(t *testing.T) {
		custom := "my custom coordinator"
		got := CoordinatorPromptForTier(custom, TierSmall)
		if got != custom {
			t.Error("expected custom prompt to be preserved")
		}
	})
}

func TestAutonomousPromptForTier(t *testing.T) {
	t.Run("large returns full", func(t *testing.T) {
		got := AutonomousPromptForTier(AutonomousSystemPrompt, TierLarge)
		if got != AutonomousSystemPrompt {
			t.Error("expected full prompt for TierLarge")
		}
	})
	t.Run("small returns compact", func(t *testing.T) {
		got := AutonomousPromptForTier(AutonomousSystemPrompt, TierSmall)
		if got != AutonomousSystemPromptCompact {
			t.Error("expected compact prompt for TierSmall")
		}
	})
	t.Run("small with custom returns custom", func(t *testing.T) {
		custom := "my custom autonomous"
		got := AutonomousPromptForTier(custom, TierSmall)
		if got != custom {
			t.Error("expected custom prompt to be preserved")
		}
	})
}
