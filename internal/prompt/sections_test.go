package prompt

import (
	"strings"
	"testing"
)

func TestToolSection_ActiveOnly(t *testing.T) {
	result := ToolSection(
		[]string{"b_tool", "a_tool"},
		map[string]string{"a_tool": "desc a", "b_tool": "desc b", "c_tool": "desc c"},
		false,
	)

	if !strings.Contains(result, "a_tool, b_tool") {
		t.Error("expected sorted active tools")
	}
	if !strings.Contains(result, "c_tool") {
		t.Error("expected inactive tool c_tool")
	}
}

func TestToolSection_Compact_SkipsInactive(t *testing.T) {
	result := ToolSection(
		[]string{"a_tool"},
		map[string]string{"a_tool": "desc a", "b_tool": "desc b"},
		true,
	)

	if strings.Contains(result, "Additional Tools") {
		t.Error("compact mode should skip inactive tools section")
	}
	if !strings.Contains(result, "a_tool") {
		t.Error("expected active tool")
	}
}

func TestSessionSection_Full(t *testing.T) {
	result := SessionSection("/home/user/project", "fr", "My Session", 10)

	if !strings.Contains(result, "/home/user/project") {
		t.Error("expected root dir")
	}
	if !strings.Contains(result, "fr") {
		t.Error("expected language")
	}
	if !strings.Contains(result, "My Session") {
		t.Error("expected title")
	}
	if !strings.Contains(result, "10 previous messages") {
		t.Error("expected message count")
	}
}

func TestSessionSection_Empty(t *testing.T) {
	result := SessionSection("", "", "", 0)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestSessionSection_TitleHiddenWithoutMessages(t *testing.T) {
	result := SessionSection("", "", "Title", 0)
	if strings.Contains(result, "Title") {
		t.Error("title should not appear when msgCount is 0")
	}
}

func TestSkillSection_Empty(t *testing.T) {
	result := SkillSection(nil, false)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestSkillSection_Compact_LimitsFive(t *testing.T) {
	skills := map[string]string{
		"a": "desc", "b": "desc", "c": "desc",
		"d": "desc", "e": "desc", "f": "desc", "g": "desc",
	}
	result := SkillSection(skills, true)

	count := strings.Count(result, "- **")
	if count != 5 {
		t.Errorf("expected 5 skills in compact mode, got %d", count)
	}
}

func TestSkillSection_Full_ShowsAll(t *testing.T) {
	skills := map[string]string{
		"a": "desc", "b": "desc", "c": "desc",
		"d": "desc", "e": "desc", "f": "desc", "g": "desc",
	}
	result := SkillSection(skills, false)

	count := strings.Count(result, "- **")
	if count != 7 {
		t.Errorf("expected 7 skills, got %d", count)
	}
}

func TestActorSection_Empty(t *testing.T) {
	result := ActorSection(nil)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestActorSection_WithActors(t *testing.T) {
	actors := []ActorInfo{
		{Name: "coder", Tags: []string{"code"}, Capabilities: []string{"go"}},
		{Name: "writer", Tags: []string{"text"}, PromptPrefix: strings.Repeat("x", 100)},
	}
	result := ActorSection(actors)

	if !strings.Contains(result, "**coder**") {
		t.Error("expected coder actor")
	}
	if !strings.Contains(result, "**writer**") {
		t.Error("expected writer actor")
	}
	// PromptPrefix should be truncated at 80
	if !strings.Contains(result, "...") {
		t.Error("expected truncated prompt prefix")
	}
}

func TestMemorySection_Empty(t *testing.T) {
	result := MemorySection(nil, 0)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestMemorySection_WithTruncation(t *testing.T) {
	memories := []MemoryInfo{
		{Type: "fact", Title: "Go version", Content: "Go 1.25 is the latest"},
	}
	result := MemorySection(memories, 10)

	if !strings.Contains(result, "Go 1.25 is...") {
		t.Errorf("expected truncated content, got %q", result)
	}
}

func TestMemorySection_NoTruncation(t *testing.T) {
	memories := []MemoryInfo{
		{Type: "fact", Title: "Go version", Content: "Go 1.25"},
	}
	result := MemorySection(memories, 0)

	if !strings.Contains(result, "Go 1.25") {
		t.Error("expected full content")
	}
	if strings.Contains(result, "...") {
		t.Error("should not truncate with contentMax=0")
	}
}

func TestLanguageSection_Empty(t *testing.T) {
	result := LanguageSection("")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestLanguageSection_French(t *testing.T) {
	result := LanguageSection("fr")
	if !strings.Contains(result, "French") {
		t.Errorf("expected French, got %q", result)
	}
}

func TestLanguageSection_Unknown(t *testing.T) {
	result := LanguageSection("xyz")
	if !strings.Contains(result, "xyz") {
		t.Errorf("expected raw code passthrough, got %q", result)
	}
}

func TestCustomInstructionSection_Empty(t *testing.T) {
	result := CustomInstructionSection("")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestCustomInstructionSection_WithContent(t *testing.T) {
	result := CustomInstructionSection("Be concise.")
	if !strings.Contains(result, "## Additional Instructions") {
		t.Error("expected header")
	}
	if !strings.Contains(result, "Be concise.") {
		t.Error("expected content")
	}
}

func TestRuntimeSection_Container(t *testing.T) {
	tools := []SystemTool{
		{Name: "git", Version: "git version 2.43.0"},
		{Name: "docker", Version: "Docker version 27.0.0"},
	}
	result := RuntimeSection("container", tools)

	if !strings.Contains(result, "**container** mode") {
		t.Error("expected container mode mention")
	}
	if !strings.Contains(result, "git (git version 2.43.0)") {
		t.Error("expected git tool listing")
	}
}

func TestRuntimeSection_Local_NoTools(t *testing.T) {
	result := RuntimeSection("local", nil)
	if result != "" {
		t.Errorf("expected empty for local without tools, got %q", result)
	}
}

func TestRuntimeSection_Local_WithTools(t *testing.T) {
	tools := []SystemTool{{Name: "git", Version: "2.43.0"}}
	result := RuntimeSection("local", tools)

	if strings.Contains(result, "container") {
		t.Error("should not mention container in local mode")
	}
	if !strings.Contains(result, "System Tools Available") {
		t.Error("expected tools header")
	}
}
