package agent

import (
	"strings"
	"testing"

	"github.com/dohr-michael/ozzie/internal/sessions"
)

func TestCompose_EmptyContext(t *testing.T) {
	pc := NewPromptComposer()
	result := pc.Compose(PromptContext{})
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestCompose_CustomInstructionsOnly(t *testing.T) {
	pc := NewPromptComposer()
	result := pc.Compose(PromptContext{
		CustomInstructions: "Always respond in French.",
	})
	if !strings.Contains(result, "Always respond in French.") {
		t.Errorf("expected custom instructions in output, got %q", result)
	}
	if !strings.Contains(result, "## Additional Instructions") {
		t.Errorf("expected section header, got %q", result)
	}
}

func TestCompose_ToolsManifest(t *testing.T) {
	pc := NewPromptComposer()
	result := pc.Compose(PromptContext{
		ToolNames: []string{"root_cmd", "cmd"},
		ToolDescriptions: map[string]string{
			"cmd":      "Execute a shell command",
			"root_cmd": "Execute a privileged command",
		},
	})
	if !strings.Contains(result, "## Available Tools") {
		t.Errorf("expected tools section header, got %q", result)
	}
	if !strings.Contains(result, "**cmd**: Execute a shell command") {
		t.Errorf("expected cmd tool entry, got %q", result)
	}
	if !strings.Contains(result, "**root_cmd**: Execute a privileged command") {
		t.Errorf("expected root_cmd tool entry, got %q", result)
	}
	// Verify sorted order: cmd before root_cmd
	cmdIdx := strings.Index(result, "**cmd**")
	rootIdx := strings.Index(result, "**root_cmd**")
	if cmdIdx > rootIdx {
		t.Errorf("expected tools sorted alphabetically, cmd at %d, root_cmd at %d", cmdIdx, rootIdx)
	}
}

func TestCompose_ToolWithoutDescription(t *testing.T) {
	pc := NewPromptComposer()
	result := pc.Compose(PromptContext{
		ToolNames:        []string{"mystery"},
		ToolDescriptions: map[string]string{},
	})
	if !strings.Contains(result, "- **mystery**\n") {
		t.Errorf("expected tool without description, got %q", result)
	}
}

func TestCompose_SessionContextResumed(t *testing.T) {
	pc := NewPromptComposer()
	result := pc.Compose(PromptContext{
		Session: &sessions.Session{
			Title: "Debug login bug",
		},
		MessageCount: 5,
	})
	if !strings.Contains(result, "## Session Context") {
		t.Errorf("expected session section header, got %q", result)
	}
	if !strings.Contains(result, `"Debug login bug"`) {
		t.Errorf("expected session title, got %q", result)
	}
	if !strings.Contains(result, "5 previous messages") {
		t.Errorf("expected message count, got %q", result)
	}
}

func TestCompose_SessionContextNewSession(t *testing.T) {
	pc := NewPromptComposer()
	result := pc.Compose(PromptContext{
		Session:      &sessions.Session{Title: "New chat"},
		MessageCount: 0,
	})
	if strings.Contains(result, "## Session Context") {
		t.Errorf("expected no session section for 0 messages, got %q", result)
	}
}

func TestCompose_SkillDescriptions(t *testing.T) {
	pc := NewPromptComposer()
	result := pc.Compose(PromptContext{
		SkillDescriptions: map[string]string{
			"researcher":  "Deep web research and synthesis",
			"code-review": "Automated code review",
		},
	})

	if !strings.Contains(result, "## Available Skills") {
		t.Errorf("expected skills section header, got %q", result)
	}
	if !strings.Contains(result, "**code-review**: Automated code review") {
		t.Errorf("expected code-review skill, got %q", result)
	}
	if !strings.Contains(result, "**researcher**: Deep web research") {
		t.Errorf("expected researcher skill, got %q", result)
	}
	// Verify sorted order: code-review before researcher
	crIdx := strings.Index(result, "**code-review**")
	rIdx := strings.Index(result, "**researcher**")
	if crIdx > rIdx {
		t.Errorf("expected skills sorted alphabetically, code-review at %d, researcher at %d", crIdx, rIdx)
	}
}

func TestCompose_AllLayers(t *testing.T) {
	pc := NewPromptComposer()
	result := pc.Compose(PromptContext{
		CustomInstructions: "Be concise.",
		ToolNames:          []string{"cmd"},
		ToolDescriptions:   map[string]string{"cmd": "Run a command"},
		SkillDescriptions:  map[string]string{"summarizer": "Summarize text"},
		Session:            &sessions.Session{Title: "My session"},
		MessageCount:       10,
		TaskInstructions:   "Step 3: validate output",
	})

	// All sections present
	for _, section := range []string{
		"## Additional Instructions",
		"## Available Tools",
		"## Available Skills",
		"## Session Context",
		"## Current Task",
	} {
		if !strings.Contains(result, section) {
			t.Errorf("expected section %q, got %q", section, result)
		}
	}

	// Correct order
	instrIdx := strings.Index(result, "## Additional Instructions")
	toolsIdx := strings.Index(result, "## Available Tools")
	skillsIdx := strings.Index(result, "## Available Skills")
	sessIdx := strings.Index(result, "## Session Context")
	taskIdx := strings.Index(result, "## Current Task")

	if instrIdx > toolsIdx || toolsIdx > skillsIdx || skillsIdx > sessIdx || sessIdx > taskIdx {
		t.Errorf("sections not in expected order: instructions=%d, tools=%d, skills=%d, session=%d, task=%d",
			instrIdx, toolsIdx, skillsIdx, sessIdx, taskIdx)
	}
}
