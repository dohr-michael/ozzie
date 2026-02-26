package agent

import (
	"strings"
	"testing"
)

func TestSubAgentInstructions_ContainsKeySections(t *testing.T) {
	sections := []string{
		"## Operating Mode",
		"## Tool Reference",
		"## Workflow",
	}
	for _, s := range sections {
		if !strings.Contains(SubAgentInstructions, s) {
			t.Errorf("SubAgentInstructions missing section %q", s)
		}
	}
}

func TestSubAgentInstructions_ContainsToolNames(t *testing.T) {
	tools := []string{"ls", "read_file", "write_file", "edit_file", "run_command", "query_memories"}
	for _, tool := range tools {
		if !strings.Contains(SubAgentInstructions, tool) {
			t.Errorf("SubAgentInstructions missing tool %q", tool)
		}
	}
}

func TestSubAgentInstructions_ContainsActionDirective(t *testing.T) {
	if !strings.Contains(SubAgentInstructions, "actually call the tools") {
		t.Error("SubAgentInstructions missing action directive")
	}
}

func TestAgentInstructions_ContainsMemoryProtocol(t *testing.T) {
	keywords := []string{"Memory Protocol", "query_memories", "store_memory"}
	for _, kw := range keywords {
		if !strings.Contains(AgentInstructions, kw) {
			t.Errorf("AgentInstructions missing keyword %q", kw)
		}
	}
}

func TestNewSubAgentMiddleware_InjectsInstructions(t *testing.T) {
	mw := NewSubAgentMiddleware()
	if mw.AdditionalInstruction != SubAgentInstructions {
		t.Errorf("middleware AdditionalInstruction = %q, want SubAgentInstructions", mw.AdditionalInstruction)
	}
}
