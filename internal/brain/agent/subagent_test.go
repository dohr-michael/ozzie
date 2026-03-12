package agent

import (
	"strings"
	"testing"

	"github.com/dohr-michael/ozzie/internal/prompt"
)

func TestSubAgentInstructions_ContainsKeySections(t *testing.T) {
	sections := []string{
		"## Operating Mode",
		"## Tool Reference",
		"## Workflow",
	}
	for _, s := range sections {
		if !strings.Contains(prompt.SubAgentInstructions, s) {
			t.Errorf("SubAgentInstructions missing section %q", s)
		}
	}
}

func TestSubAgentInstructions_ContainsToolNames(t *testing.T) {
	tools := []string{"ls", "read_file", "write_file", "edit_file", "run_command", "query_memories"}
	for _, tool := range tools {
		if !strings.Contains(prompt.SubAgentInstructions, tool) {
			t.Errorf("SubAgentInstructions missing tool %q", tool)
		}
	}
}

func TestSubAgentInstructions_ContainsActionDirective(t *testing.T) {
	if !strings.Contains(prompt.SubAgentInstructions, "actually call the tools") {
		t.Error("SubAgentInstructions missing action directive")
	}
}

func TestAgentInstructions_ContainsMemoryProtocol(t *testing.T) {
	keywords := []string{"Memory Protocol", "query_memories", "store_memory"}
	for _, kw := range keywords {
		if !strings.Contains(prompt.AgentInstructions, kw) {
			t.Errorf("AgentInstructions missing keyword %q", kw)
		}
	}
}

func TestNewSubAgentMiddleware_InjectsInstructions(t *testing.T) {
	mw := NewSubAgentMiddleware("", TierLarge)
	if mw.AdditionalInstruction != prompt.SubAgentInstructions {
		t.Errorf("middleware AdditionalInstruction = %q, want SubAgentInstructions", mw.AdditionalInstruction)
	}
}

func TestNewSubAgentMiddleware_WithRuntimeInstruction(t *testing.T) {
	runtime := "## Runtime Environment\n\nYou are running in **container** mode."
	mw := NewSubAgentMiddleware(runtime, TierLarge)
	if !strings.Contains(mw.AdditionalInstruction, prompt.SubAgentInstructions) {
		t.Error("middleware should contain SubAgentInstructions")
	}
	if !strings.Contains(mw.AdditionalInstruction, runtime) {
		t.Error("middleware should contain runtime instruction")
	}
}

func TestNewSubAgentMiddleware_TierSmall(t *testing.T) {
	mw := NewSubAgentMiddleware("", TierSmall)
	if mw.AdditionalInstruction != prompt.SubAgentInstructionsCompact {
		t.Errorf("expected compact instructions for TierSmall")
	}
}
