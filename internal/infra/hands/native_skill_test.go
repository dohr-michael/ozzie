package hands

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/dohr-michael/ozzie/internal/core/events"
)

// withSession injects a session ID into the context for testing.
func withSession(ctx context.Context, id string) context.Context {
	return events.ContextWithSessionID(ctx, id)
}

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

// --- activate tool tests ---

func TestActivateTool_Info(t *testing.T) {
	tool := NewActivateTool(newMockActivator(nil), &ToolRegistry{}, &mockSkillCatalog{})
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != ToolActivate {
		t.Errorf("expected name %q, got %q", ToolActivate, info.Name)
	}
}

func TestActivateTool_ActivateSkill(t *testing.T) {
	catalog := &mockSkillCatalog{
		skills: map[string]mockSkillEntry{
			"analyst": {
				body:  "# Analyst\n\nYou are a business analyst.",
				tools: []string{"read_file"},
			},
		},
	}
	activator := newMockActivator([]string{"read_file"})

	tool := NewActivateTool(activator, &ToolRegistry{}, catalog)

	args, _ := json.Marshal(activateInput{Names: []string{"analyst"}})
	result, err := tool.InvokableRun(
		withSession(context.Background(), "sess_test"),
		string(args),
	)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	var out activateOutput
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(out.Activated) != 1 {
		t.Fatalf("expected 1 activated, got %d", len(out.Activated))
	}
	if out.Activated[0].Type != "skill" {
		t.Errorf("expected type 'skill', got %q", out.Activated[0].Type)
	}
	if out.Activated[0].Body != "# Analyst\n\nYou are a business analyst." {
		t.Errorf("unexpected body: %q", out.Activated[0].Body)
	}
}

func TestActivateTool_ActivateTool(t *testing.T) {
	activator := newMockActivator([]string{"docker_build"})
	reg := &ToolRegistry{}

	tool := NewActivateTool(activator, reg, &mockSkillCatalog{})

	args, _ := json.Marshal(activateInput{Names: []string{"docker_build"}})
	result, err := tool.InvokableRun(
		withSession(context.Background(), "sess_test"),
		string(args),
	)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	var out activateOutput
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(out.Activated) != 1 {
		t.Fatalf("expected 1 activated, got %d", len(out.Activated))
	}
	if out.Activated[0].Type != "tool" {
		t.Errorf("expected type 'tool', got %q", out.Activated[0].Type)
	}
}

func TestActivateTool_Unknown(t *testing.T) {
	tool := NewActivateTool(newMockActivator(nil), &ToolRegistry{}, &mockSkillCatalog{})

	args, _ := json.Marshal(activateInput{Names: []string{"nonexistent"}})
	result, err := tool.InvokableRun(
		withSession(context.Background(), "sess_test"),
		string(args),
	)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	var out activateOutput
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(out.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(out.Errors))
	}
}

func TestActivateTool_EmptyNames(t *testing.T) {
	tool := NewActivateTool(newMockActivator(nil), &ToolRegistry{}, &mockSkillCatalog{})

	args, _ := json.Marshal(activateInput{Names: []string{}})
	_, err := tool.InvokableRun(
		withSession(context.Background(), "sess_test"),
		string(args),
	)
	if err == nil {
		t.Fatal("expected error for empty names")
	}
}

// --- run_workflow tests ---

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
