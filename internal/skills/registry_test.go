package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	skill := &Skill{
		Name:        "test-skill",
		Description: "A test skill",
		Type:        SkillTypeSimple,
		Instruction: "Do the thing",
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

	skill := &Skill{
		Name:        "dup",
		Description: "A skill",
		Type:        SkillTypeSimple,
		Instruction: "Do it",
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
		_ = r.Register(&Skill{
			Name:        name,
			Description: "desc",
			Type:        SkillTypeSimple,
			Instruction: "do it",
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

func TestRegistry_LoadDir(t *testing.T) {
	dir := t.TempDir()

	// Write a valid skill
	skillJSON := `{
		"name": "summarizer",
		"description": "Summarize text concisely",
		"instruction": "You are a concise summarizer.",
		"triggers": { "delegation": true }
	}`
	if err := os.WriteFile(filepath.Join(dir, "summarizer.jsonc"), []byte(skillJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write an invalid skill (missing name)
	invalidJSON := `{ "description": "bad" }`
	if err := os.WriteFile(filepath.Join(dir, "invalid.jsonc"), []byte(invalidJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a non-jsonc file (should be ignored)
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

func TestRegistry_LoadDir_MissingDir(t *testing.T) {
	r := NewRegistry()
	err := r.LoadDir("/nonexistent/path/skills")
	if err != nil {
		t.Errorf("expected nil for missing dir, got: %v", err)
	}
}
