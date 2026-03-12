package hands

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// --- mock implementations ---

type mockSkillCatalog struct {
	skills map[string]mockSkillEntry
}

type mockSkillEntry struct {
	body        string
	tools       []string
	hasWorkflow bool
	dir         string
}

func (m *mockSkillCatalog) SkillBody(name string) (string, []string, bool, string, error) {
	entry, ok := m.skills[name]
	if !ok {
		return "", nil, false, "", fmt.Errorf("skill not found: %s", name)
	}
	return entry.body, entry.tools, entry.hasWorkflow, entry.dir, nil
}

func (m *mockSkillCatalog) Names() []string {
	var names []string
	for name := range m.skills {
		names = append(names, name)
	}
	return names
}

type mockWorkflowExecutor struct {
	result string
	err    error
}

func (m *mockWorkflowExecutor) RunWorkflow(_ context.Context, _ string, _ map[string]string) (string, error) {
	return m.result, m.err
}

// --- tests ---

func TestActivateSkillTool_Info(t *testing.T) {
	tool := NewActivateSkillTool(&mockSkillCatalog{}, &mockActivator{}, &ToolRegistry{})
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "activate_skill" {
		t.Errorf("expected name %q, got %q", "activate_skill", info.Name)
	}
}

func TestActivateSkillTool_Run(t *testing.T) {
	catalog := &mockSkillCatalog{
		skills: map[string]mockSkillEntry{
			"analyst": {
				body:  "# Analyst\n\nYou are a business analyst.",
				tools: []string{"read_file"},
			},
		},
	}
	activator := &mockActivator{known: map[string]bool{"read_file": true}}

	tool := NewActivateSkillTool(catalog, activator, &ToolRegistry{})

	args, _ := json.Marshal(activateSkillInput{Name: "analyst"})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	var out activateSkillOutput
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out.Body != "# Analyst\n\nYou are a business analyst." {
		t.Errorf("unexpected body: %q", out.Body)
	}
	if out.HasWorkflow {
		t.Error("expected no workflow")
	}
}

func TestActivateSkillTool_NotFound(t *testing.T) {
	catalog := &mockSkillCatalog{skills: map[string]mockSkillEntry{}}
	tool := NewActivateSkillTool(catalog, &mockActivator{}, &ToolRegistry{})

	args, _ := json.Marshal(activateSkillInput{Name: "nonexistent"})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
}

func TestActivateSkillTool_EmptyName(t *testing.T) {
	tool := NewActivateSkillTool(&mockSkillCatalog{}, &mockActivator{}, &ToolRegistry{})

	args, _ := json.Marshal(activateSkillInput{Name: ""})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRunWorkflowTool_Info(t *testing.T) {
	tool := NewRunWorkflowTool(&mockWorkflowExecutor{})
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "run_workflow" {
		t.Errorf("expected name %q, got %q", "run_workflow", info.Name)
	}
}

func TestRunWorkflowTool_Run(t *testing.T) {
	executor := &mockWorkflowExecutor{result: "workflow completed"}
	tool := NewRunWorkflowTool(executor)

	args, _ := json.Marshal(runWorkflowInput{
		SkillName: "builder",
		Vars:      map[string]string{"task": "build it"},
	})

	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	var out map[string]string
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out["skill"] != "builder" {
		t.Errorf("unexpected skill: %q", out["skill"])
	}
	if out["output"] != "workflow completed" {
		t.Errorf("unexpected output: %q", out["output"])
	}
}

func TestRunWorkflowTool_EmptyName(t *testing.T) {
	tool := NewRunWorkflowTool(&mockWorkflowExecutor{})

	args, _ := json.Marshal(runWorkflowInput{SkillName: ""})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("expected error for empty skill_name")
	}
}

func TestRunWorkflowTool_ExecutorError(t *testing.T) {
	executor := &mockWorkflowExecutor{err: fmt.Errorf("DAG failed")}
	tool := NewRunWorkflowTool(executor)

	args, _ := json.Marshal(runWorkflowInput{SkillName: "broken"})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("expected error from executor")
	}
}
