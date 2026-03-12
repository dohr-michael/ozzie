package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitFrontmatter(t *testing.T) {
	input := []byte("---\nname: test\n---\n# Hello\n\nBody text.")
	fm, body, err := splitFrontmatter(input)
	if err != nil {
		t.Fatalf("splitFrontmatter: %v", err)
	}
	if !strings.Contains(string(fm), "name: test") {
		t.Errorf("expected frontmatter to contain 'name: test', got %q", string(fm))
	}
	if !strings.Contains(body, "# Hello") {
		t.Errorf("expected body to contain '# Hello', got %q", body)
	}
	if !strings.Contains(body, "Body text.") {
		t.Errorf("expected body to contain 'Body text.', got %q", body)
	}
}

func TestSplitFrontmatter_NoOpening(t *testing.T) {
	_, _, err := splitFrontmatter([]byte("no frontmatter here"))
	if err == nil {
		t.Fatal("expected error for missing opening ---")
	}
}

func TestSplitFrontmatter_NoClosing(t *testing.T) {
	_, _, err := splitFrontmatter([]byte("---\nname: test\n"))
	if err == nil {
		t.Fatal("expected error for missing closing ---")
	}
}

func TestParseSkillMD(t *testing.T) {
	data := []byte(`---
name: analyst
description: Analyze requirements
allowed-tools:
  - read_file
  - search
---
# Analyst

You are a business analyst.
`)

	skill, err := ParseSkillMD(data)
	if err != nil {
		t.Fatalf("ParseSkillMD: %v", err)
	}
	if skill.Name != "analyst" {
		t.Errorf("expected name %q, got %q", "analyst", skill.Name)
	}
	if skill.Description != "Analyze requirements" {
		t.Errorf("expected description %q, got %q", "Analyze requirements", skill.Description)
	}
	if len(skill.AllowedTools) != 2 {
		t.Fatalf("expected 2 allowed-tools, got %d", len(skill.AllowedTools))
	}
	if skill.AllowedTools[0] != "read_file" {
		t.Errorf("expected first tool %q, got %q", "read_file", skill.AllowedTools[0])
	}
	if !strings.Contains(skill.Body, "# Analyst") {
		t.Errorf("expected body to contain '# Analyst', got %q", skill.Body)
	}
}

func TestParseSkillMD_AllowedToolsSpaceSeparated(t *testing.T) {
	data := []byte(`---
name: test
description: Test skill
allowed-tools: read_file search web_fetch
---
Body.
`)

	skill, err := ParseSkillMD(data)
	if err != nil {
		t.Fatalf("ParseSkillMD: %v", err)
	}
	if len(skill.AllowedTools) != 3 {
		t.Fatalf("expected 3 allowed-tools, got %d: %v", len(skill.AllowedTools), skill.AllowedTools)
	}
	if skill.AllowedTools[0] != "read_file" || skill.AllowedTools[1] != "search" || skill.AllowedTools[2] != "web_fetch" {
		t.Errorf("unexpected tools: %v", skill.AllowedTools)
	}
}

func TestLoadSkillDir(t *testing.T) {
	dir := t.TempDir()

	// Write SKILL.md
	skillMD := `---
name: hello
description: Say hello
---
# Hello Skill

Greet the user warmly.
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	skill, err := LoadSkillDir(dir)
	if err != nil {
		t.Fatalf("LoadSkillDir: %v", err)
	}
	if skill.Name != "hello" {
		t.Errorf("expected name %q, got %q", "hello", skill.Name)
	}
	if skill.Dir != dir {
		t.Errorf("expected dir %q, got %q", dir, skill.Dir)
	}
	if !strings.Contains(skill.Body, "Greet the user warmly") {
		t.Errorf("expected body to contain greeting instruction, got %q", skill.Body)
	}
	if skill.HasWorkflow() {
		t.Error("expected no workflow")
	}
}

func TestLoadSkillDir_WithWorkflow(t *testing.T) {
	dir := t.TempDir()

	skillMD := `---
name: builder
description: Build things
---
Follow the workflow steps.
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	workflow := `model: claude-sonnet
vars:
  task:
    description: The task to do
    required: true
steps:
  - id: analyze
    title: Analyze
    instruction: Analyze the requirements.
  - id: build
    title: Build
    instruction: Build the thing.
    needs: [analyze]
`
	if err := os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}

	skill, err := LoadSkillDir(dir)
	if err != nil {
		t.Fatalf("LoadSkillDir: %v", err)
	}
	if !skill.HasWorkflow() {
		t.Fatal("expected workflow")
	}
	if len(skill.Workflow.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(skill.Workflow.Steps))
	}
	if skill.Workflow.Vars["task"].Required != true {
		t.Error("expected task var to be required")
	}
}

func TestLoadSkillDir_WithTriggers(t *testing.T) {
	dir := t.TempDir()

	skillMD := `---
name: watcher
description: Watch for events
---
React to events.
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	triggers := `delegation: true
cron: "0 * * * *"
keywords: [deploy, release]
`
	if err := os.WriteFile(filepath.Join(dir, "triggers.yaml"), []byte(triggers), 0o644); err != nil {
		t.Fatal(err)
	}

	skill, err := LoadSkillDir(dir)
	if err != nil {
		t.Fatalf("LoadSkillDir: %v", err)
	}
	if skill.Triggers == nil {
		t.Fatal("expected triggers")
	}
	if !skill.Triggers.HasScheduleTrigger() {
		t.Error("expected schedule trigger")
	}
	if skill.Triggers.Cron != "0 * * * *" {
		t.Errorf("expected cron %q, got %q", "0 * * * *", skill.Triggers.Cron)
	}
	if len(skill.Triggers.Keywords) != 2 {
		t.Fatalf("expected 2 keywords, got %d", len(skill.Triggers.Keywords))
	}
}

func TestLoadSkillDir_MissingSKILLMD(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadSkillDir(dir)
	if err == nil {
		t.Fatal("expected error for missing SKILL.md")
	}
}

func TestLoadSkillDir_ValidationError(t *testing.T) {
	dir := t.TempDir()

	// Missing description
	skillMD := `---
name: broken
---
Body.
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadSkillDir(dir)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "description is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadSkillDir_WorkflowValidation(t *testing.T) {
	dir := t.TempDir()

	skillMD := `---
name: bad-wf
description: Bad workflow
---
Body.
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	// Step with unknown dependency
	workflow := `steps:
  - id: s1
    instruction: Do something.
    needs: [nonexistent]
`
	if err := os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadSkillDir(dir)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "unknown step") {
		t.Errorf("unexpected error: %v", err)
	}
}
