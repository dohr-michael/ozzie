package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dohr-michael/ozzie/internal/sessions"
)

// PromptContext holds dynamic context for per-turn prompt composition.
type PromptContext struct {
	CustomInstructions string            // Layer 2: from config
	ToolNames          []string          // Layer 3: active tool names
	ToolDescriptions   map[string]string // Layer 3: tool name → description
	SkillDescriptions  map[string]string // Layer 3b: skill name → description
	Session            *sessions.Session // Layer 4: session metadata
	MessageCount       int               // Layer 4: nb messages in history
	TaskInstructions   string            // Layer 5: stub for future
}

// PromptComposer builds dynamic context layers (2-5).
type PromptComposer struct{}

// NewPromptComposer creates a new PromptComposer.
func NewPromptComposer() *PromptComposer {
	return &PromptComposer{}
}

// Compose builds a dynamic system prompt from the given context layers.
// Returns "" if all layers are empty (backward compatible).
func (pc *PromptComposer) Compose(pctx PromptContext) string {
	var sections []string

	// Layer 2: Custom instructions
	if pctx.CustomInstructions != "" {
		sections = append(sections, "## Additional Instructions\n\n"+pctx.CustomInstructions)
	}

	// Layer 3: Active tools manifest
	if len(pctx.ToolNames) > 0 {
		sorted := make([]string, len(pctx.ToolNames))
		copy(sorted, pctx.ToolNames)
		sort.Strings(sorted)

		var sb strings.Builder
		sb.WriteString("## Available Tools\n\n")
		sb.WriteString("You have access to the following tools:\n")
		for _, name := range sorted {
			if desc, ok := pctx.ToolDescriptions[name]; ok && desc != "" {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", name, desc))
			} else {
				sb.WriteString(fmt.Sprintf("- **%s**\n", name))
			}
		}
		sections = append(sections, sb.String())
	}

	// Layer 3b: Available skills
	if len(pctx.SkillDescriptions) > 0 {
		names := make([]string, 0, len(pctx.SkillDescriptions))
		for name := range pctx.SkillDescriptions {
			names = append(names, name)
		}
		sort.Strings(names)

		var sb strings.Builder
		sb.WriteString("## Available Skills\n\n")
		sb.WriteString("You can delegate complex tasks to these specialized skills:\n")
		for _, name := range names {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", name, pctx.SkillDescriptions[name]))
		}
		sections = append(sections, sb.String())
	}

	// Layer 4: Session context
	if pctx.Session != nil && pctx.MessageCount > 0 {
		var sb strings.Builder
		sb.WriteString("## Session Context\n\n")
		if pctx.Session.Title != "" {
			sb.WriteString(fmt.Sprintf("Resumed session: %q.\n", pctx.Session.Title))
		} else {
			sb.WriteString("Resumed session.\n")
		}
		sb.WriteString(fmt.Sprintf("%d previous messages.", pctx.MessageCount))
		sections = append(sections, sb.String())
	}

	// Layer 5: Task-specific instructions (stub for future)
	if pctx.TaskInstructions != "" {
		sections = append(sections, "## Current Task\n\n"+pctx.TaskInstructions)
	}

	if len(sections) == 0 {
		return ""
	}

	return strings.Join(sections, "\n\n")
}
