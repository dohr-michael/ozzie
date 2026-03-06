package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	skill := &SkillMD{
		Name:        "test-skill",
		Description: "A test skill",
		Body:        "Do the thing.",
	}

	if err := r.Register(skill); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := r.Get("test-skill")
	if got == nil {
		t.Fatal("expected skill, got nil")
	}
	if got.Name != "test-skill" {
		t.Errorf("expected name 'test-skill', got %q", got.Name)
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry()

	skill := &SkillMD{
		Name:        "dup",
		Description: "A skill",
		Body:        "Do it.",
	}

	if err := r.Register(skill); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := r.Register(skill)
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	r := NewRegistry()
	if r.Get("nope") != nil {
		t.Error("expected nil for missing skill")
	}
}

func TestRegistry_AllAndNames(t *testing.T) {
	r := NewRegistry()

	for _, name := range []string{"charlie", "alpha", "bravo"} {
		_ = r.Register(&SkillMD{
			Name:        name,
			Description: "desc",
			Body:        "do it",
		})
	}

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(all))
	}
	// Sorted
	if all[0].Name != "alpha" || all[1].Name != "bravo" || all[2].Name != "charlie" {
		t.Errorf("unexpected order: %v, %v, %v", all[0].Name, all[1].Name, all[2].Name)
	}

	names := r.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "bravo" || names[2] != "charlie" {
		t.Errorf("unexpected name order: %v", names)
	}
}

func TestRegistry_Catalog(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&SkillMD{Name: "a", Description: "desc-a", Body: "body"})
	_ = r.Register(&SkillMD{Name: "b", Description: "desc-b", Body: "body"})

	catalog := r.Catalog()
	if len(catalog) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(catalog))
	}
	if catalog["a"] != "desc-a" {
		t.Errorf("expected desc-a, got %q", catalog["a"])
	}
}

func TestRegistry_SkillBody(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&SkillMD{
		Name:         "test",
		Description:  "desc",
		Body:         "the body",
		AllowedTools: []string{"read_file"},
		Dir:          "/tmp/test",
	})

	body, tools, hasWf, dir, err := r.SkillBody("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != "the body" {
		t.Errorf("expected body %q, got %q", "the body", body)
	}
	if len(tools) != 1 || tools[0] != "read_file" {
		t.Errorf("unexpected tools: %v", tools)
	}
	if hasWf {
		t.Error("expected no workflow")
	}
	if dir != "/tmp/test" {
		t.Errorf("expected dir %q, got %q", "/tmp/test", dir)
	}

	_, _, _, _, err = r.SkillBody("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing skill")
	}
}

func TestRegistry_LoadDir(t *testing.T) {
	dir := t.TempDir()

	// Create a valid skill subdirectory
	skillDir := filepath.Join(dir, "summarizer")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMD := `---
name: summarizer
description: Summarize text concisely
---
You are a concise summarizer.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create an invalid skill (missing description)
	invalidDir := filepath.Join(dir, "invalid")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	invalidMD := `---
name: invalid
---
Body.
`
	if err := os.WriteFile(filepath.Join(invalidDir, "SKILL.md"), []byte(invalidMD), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a non-skill directory (no SKILL.md)
	if err := os.MkdirAll(filepath.Join(dir, "random"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "random", "readme.md"), []byte("# Random"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a plain file (not a directory)
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# skills"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	if err := r.LoadDir(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the valid skill should be loaded
	if r.Get("summarizer") == nil {
		t.Error("expected summarizer skill to be loaded")
	}
	if len(r.All()) != 1 {
		t.Errorf("expected 1 skill, got %d", len(r.All()))
	}
}

// TestLoadSkillFiles loads every SKILL.md directory from the project's examples/skills/
// directory and verifies that each one parses and validates successfully.
func TestLoadSkillFiles(t *testing.T) {
	skillsDir := filepath.Join("..", "..", "examples", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Skipf("examples/skills not found, skipping: %v", err)
	}

	var loaded int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subDir := filepath.Join(skillsDir, entry.Name())
		skillPath := filepath.Join(subDir, "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			s, err := LoadSkillDir(subDir)
			if err != nil {
				t.Fatalf("LoadSkillDir(%s): %v", entry.Name(), err)
			}
			if s.Name == "" {
				t.Fatal("skill name is empty")
			}
			if s.Body == "" && !s.HasWorkflow() {
				t.Fatal("skill has no body and no workflow")
			}
		})
		loaded++
	}

	if loaded == 0 {
		t.Skip("no SKILL.md directories found in examples/skills/")
	}
	t.Logf("loaded %d skill directories successfully", loaded)
}

func TestRegistry_LoadDir_MissingDir(t *testing.T) {
	r := NewRegistry()
	err := r.LoadDir("/nonexistent/path/skills")
	if err != nil {
		t.Errorf("expected nil for missing dir, got: %v", err)
	}
}
