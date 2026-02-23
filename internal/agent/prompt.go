package agent

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dohr-michael/ozzie/internal/sessions"
)

// PromptContext holds dynamic context for per-turn prompt composition.
type PromptContext struct {
	CustomInstructions  string            // Layer 2: from config
	ActiveToolNames     []string          // Layer 3: tools registered with Eino (active)
	AllToolDescriptions map[string]string // Layer 3: full catalog (name → desc)
	SkillDescriptions   map[string]string // Layer 3b: skill name → description
	Session             *sessions.Session // Layer 4: session metadata
	MessageCount        int               // Layer 4: nb messages in history
	TaskInstructions    string            // Layer 5: stub for future
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

	// Layer 3: Active tools + available (inactive) tools
	if len(pctx.ActiveToolNames) > 0 {
		activeSet := make(map[string]bool, len(pctx.ActiveToolNames))
		sorted := make([]string, len(pctx.ActiveToolNames))
		copy(sorted, pctx.ActiveToolNames)
		sort.Strings(sorted)
		for _, n := range sorted {
			activeSet[n] = true
		}

		var sb strings.Builder
		sb.WriteString("## Active Tools (ready to use)\n\n")
		for _, name := range sorted {
			if desc, ok := pctx.AllToolDescriptions[name]; ok && desc != "" {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", name, desc))
			} else {
				sb.WriteString(fmt.Sprintf("- **%s**\n", name))
			}
		}

		// Collect inactive tools
		var inactive []string
		for name := range pctx.AllToolDescriptions {
			if !activeSet[name] {
				inactive = append(inactive, name)
			}
		}
		sort.Strings(inactive)

		if len(inactive) > 0 {
			sb.WriteString("\n## Available Tools (call activate_tools first)\n\n")
			for _, name := range inactive {
				if desc := pctx.AllToolDescriptions[name]; desc != "" {
					sb.WriteString(fmt.Sprintf("- **%s**: %s\n", name, desc))
				} else {
					sb.WriteString(fmt.Sprintf("- **%s**\n", name))
				}
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
	if pctx.Session != nil {
		var sb strings.Builder
		sb.WriteString("## Session Context\n\n")
		if pctx.Session.RootDir != "" {
			sb.WriteString(fmt.Sprintf("Working directory: %s\n", pctx.Session.RootDir))
		}
		if pctx.Session.Language != "" {
			sb.WriteString(fmt.Sprintf("Preferred language: %s\n", pctx.Session.Language))
		}
		if pctx.Session.Title != "" && pctx.MessageCount > 0 {
			sb.WriteString(fmt.Sprintf("Resumed session: %q.\n", pctx.Session.Title))
		}
		if pctx.MessageCount > 0 {
			sb.WriteString(fmt.Sprintf("%d previous messages.", pctx.MessageCount))
		}
		// Only emit the section if there's actual content beyond the header
		content := sb.String()
		if content != "## Session Context\n\n" {
			sections = append(sections, content)
		}
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
