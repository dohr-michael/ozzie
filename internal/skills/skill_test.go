package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkillFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	return path
}

func TestLoadSkill_Simple(t *testing.T) {
	dir := t.TempDir()
	path := writeSkillFile(t, dir, "greet.jsonc", `{
		"name": "greet",
		"description": "Greets the user",
		"type": "simple",
		"instruction": "Say hello to the user.",
		"model": "claude-sonnet"
	}`)

	s, err := LoadSkill(path)
	if err != nil {
		t.Fatalf("LoadSkill: %v", err)
	}
	if s.Name != "greet" {
		t.Fatalf("expected name %q, got %q", "greet", s.Name)
	}
	if s.Type != SkillTypeSimple {
		t.Fatalf("expected type %q, got %q", SkillTypeSimple, s.Type)
	}
	if s.Instruction != "Say hello to the user." {
		t.Fatalf("expected instruction %q, got %q", "Say hello to the user.", s.Instruction)
	}
	if s.Model != "claude-sonnet" {
		t.Fatalf("expected model %q, got %q", "claude-sonnet", s.Model)
	}
	if !s.Triggers.Delegation {
		t.Fatal("expected delegation=true by default")
	}
}

func TestLoadSkill_Workflow(t *testing.T) {
	dir := t.TempDir()
	path := writeSkillFile(t, dir, "deploy.jsonc", `{
		"name": "deploy",
		"description": "Deploy pipeline",
		"type": "workflow",
		"vars": {
			"env": {"description": "Target environment", "required": true}
		},
		"steps": [
			{"id": "build", "title": "Build", "instruction": "Build the project."},
			{"id": "test", "title": "Test", "instruction": "Run tests.", "needs": ["build"]},
			{"id": "deploy", "title": "Deploy", "instruction": "Deploy to env.", "needs": ["test"]}
		]
	}`)

	s, err := LoadSkill(path)
	if err != nil {
		t.Fatalf("LoadSkill: %v", err)
	}
	if s.Type != SkillTypeWorkflow {
		t.Fatalf("expected type %q, got %q", SkillTypeWorkflow, s.Type)
	}
	if len(s.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(s.Steps))
	}
	if _, ok := s.Vars["env"]; !ok {
		t.Fatal("expected var 'env' to exist")
	}
	if !s.Vars["env"].Required {
		t.Fatal("expected var 'env' to be required")
	}
}

func TestLoadSkill_InferType(t *testing.T) {
	dir := t.TempDir()
	// No explicit type, has steps → should infer workflow
	path := writeSkillFile(t, dir, "infer.jsonc", `{
		"name": "inferred",
		"description": "Type inferred from steps",
		"steps": [
			{"id": "s1", "title": "Step 1", "instruction": "Do something."}
		]
	}`)

	s, err := LoadSkill(path)
	if err != nil {
		t.Fatalf("LoadSkill: %v", err)
	}
	if s.Type != SkillTypeWorkflow {
		t.Fatalf("expected inferred type %q, got %q", SkillTypeWorkflow, s.Type)
	}
}

func TestLoadSkill_DefaultTrigger(t *testing.T) {
	dir := t.TempDir()
	path := writeSkillFile(t, dir, "trigger.jsonc", `{
		"name": "trigger-test",
		"description": "No triggers set",
		"type": "simple",
		"instruction": "Do stuff."
	}`)

	s, err := LoadSkill(path)
	if err != nil {
		t.Fatalf("LoadSkill: %v", err)
	}
	if !s.Triggers.Delegation {
		t.Fatal("expected delegation=true by default")
	}
}

func TestValidate_MissingName(t *testing.T) {
	s := &Skill{Description: "desc", Type: SkillTypeSimple, Instruction: "do"}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected 'name is required' error, got %v", err)
	}
}

func TestValidate_MissingDescription(t *testing.T) {
	s := &Skill{Name: "test", Type: SkillTypeSimple, Instruction: "do"}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "description is required") {
		t.Fatalf("expected 'description is required' error, got %v", err)
	}
}

func TestValidate_SimpleNoInstruction(t *testing.T) {
	s := &Skill{Name: "test", Description: "desc", Type: SkillTypeSimple}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "requires an instruction") {
		t.Fatalf("expected 'requires an instruction' error, got %v", err)
	}
}

func TestValidate_WorkflowNoSteps(t *testing.T) {
	s := &Skill{Name: "test", Description: "desc", Type: SkillTypeWorkflow}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "requires at least one step") {
		t.Fatalf("expected 'requires at least one step' error, got %v", err)
	}
}

func TestValidate_DuplicateStepID(t *testing.T) {
	s := &Skill{
		Name: "test", Description: "desc", Type: SkillTypeWorkflow,
		Steps: []Step{
			{ID: "s1", Instruction: "do"},
			{ID: "s1", Instruction: "do again"},
		},
	}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate step ID") {
		t.Fatalf("expected 'duplicate step ID' error, got %v", err)
	}
}

func TestValidate_UnknownStepDep(t *testing.T) {
	s := &Skill{
		Name: "test", Description: "desc", Type: SkillTypeWorkflow,
		Steps: []Step{
			{ID: "s1", Instruction: "do", Needs: []string{"nonexistent"}},
		},
	}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "unknown step") {
		t.Fatalf("expected 'unknown step' error, got %v", err)
	}
}

func TestValidate_SelfDep(t *testing.T) {
	s := &Skill{
		Name: "test", Description: "desc", Type: SkillTypeWorkflow,
		Steps: []Step{
			{ID: "s1", Instruction: "do", Needs: []string{"s1"}},
		},
	}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "cannot depend on itself") {
		t.Fatalf("expected 'cannot depend on itself' error, got %v", err)
	}
}

func TestValidate_StepNoInstruction(t *testing.T) {
	s := &Skill{
		Name: "test", Description: "desc", Type: SkillTypeWorkflow,
		Steps: []Step{
			{ID: "s1"},
		},
	}
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "requires an instruction") {
		t.Fatalf("expected 'requires an instruction' error, got %v", err)
	}
}

func TestLoadSkill_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeSkillFile(t, dir, "bad.jsonc", `{invalid json}`)

	_, err := LoadSkill(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadSkill_MissingFile(t *testing.T) {
	_, err := LoadSkill("/nonexistent/path/skill.jsonc")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
