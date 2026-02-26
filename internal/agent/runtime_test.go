package agent

import (
	"os"
	"path/filepath"
	"strings"
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
	if tools[0].Version != "git version 2.43.0" {
		t.Errorf("unexpected version: %s", tools[0].Version)
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

func TestBuildRuntimeInstruction_Container(t *testing.T) {
	tools := []SystemTool{
		{Name: "git", Version: "git version 2.43.0"},
		{Name: "docker", Version: "Docker version 27.0.0"},
	}

	result := BuildRuntimeInstruction("container", tools)

	if !strings.Contains(result, "**container** mode") {
		t.Error("expected container mode mention")
	}
	if !strings.Contains(result, "docker run") {
		t.Error("expected docker run guidance")
	}
	if !strings.Contains(result, "git (git version 2.43.0)") {
		t.Error("expected git tool listing")
	}
	if !strings.Contains(result, "docker (Docker version 27.0.0)") {
		t.Error("expected docker tool listing")
	}
}

func TestBuildRuntimeInstruction_Local_NoTools(t *testing.T) {
	result := BuildRuntimeInstruction("local", nil)
	if result != "" {
		t.Errorf("expected empty string for local without tools, got %q", result)
	}
}

func TestBuildRuntimeInstruction_Local_WithTools(t *testing.T) {
	tools := []SystemTool{
		{Name: "git", Version: "git version 2.43.0"},
	}

	result := BuildRuntimeInstruction("local", tools)

	if strings.Contains(result, "container") {
		t.Error("should not mention container in local mode")
	}
	if !strings.Contains(result, "git (git version 2.43.0)") {
		t.Error("expected git tool listing")
	}
	if !strings.Contains(result, "System Tools Available") {
		t.Error("expected System Tools Available header")
	}
}
