package prompt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSystemTools_FileNotFound(t *testing.T) {
	tools := LoadSystemTools("/nonexistent/path/tools.json")
	if tools != nil {
		t.Errorf("expected nil for missing file, got %v", tools)
	}
}

func TestLoadSystemTools_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tools.json")
	data := `[{"name":"git","version":"git version 2.43.0"},{"name":"curl","version":"curl 8.5.0"}]`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	tools := LoadSystemTools(path)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "git" {
		t.Errorf("expected git, got %s", tools[0].Name)
	}
	if tools[1].Name != "curl" {
		t.Errorf("expected curl, got %s", tools[1].Name)
	}
}

func TestLoadSystemTools_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tools.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	tools := LoadSystemTools(path)
	if tools != nil {
		t.Errorf("expected nil for invalid JSON, got %v", tools)
	}
}

func TestDefaultRegistry_ContainsAllTemplates(t *testing.T) {
	expectedIDs := []string{
		"persona.default",
		"persona.compact",
		"instructions.agent",
		"instructions.agent.compact",
		"instructions.subagent",
		"instructions.subagent.compact",
		"extraction.lessons",
		"summarize.layered.l0",
		"summarize.layered.l1",
		"summarize.compressor",
	}

	all := DefaultRegistry.All()
	if len(all) != len(expectedIDs) {
		t.Fatalf("expected %d templates, got %d", len(expectedIDs), len(all))
	}

	for _, id := range expectedIDs {
		if DefaultRegistry.Get(id) == nil {
			t.Errorf("missing template %q", id)
		}
	}
}
