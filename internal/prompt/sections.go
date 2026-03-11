package prompt

import (
	"fmt"
	"sort"
	"strings"
)

// ActorInfo describes a configured actor overlay for the planner prompt.
type ActorInfo struct {
	Name         string
	Tags         []string
	Capabilities []string
	PromptPrefix string // first 80 chars for context
}

// MemoryInfo holds a single memory entry for prompt injection.
type MemoryInfo struct {
	Type    string
	Title   string
	Content string
}

// SystemTool describes a tool available in the runtime environment.
type SystemTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolSection formats active and inactive tool lists for prompt injection.
// When compact is true, the inactive section is omitted to save tokens.
func ToolSection(activeNames []string, allDescs map[string]string, compact bool) string {
	activeSet := make(map[string]bool, len(activeNames))
	sorted := make([]string, len(activeNames))
	copy(sorted, activeNames)
	sort.Strings(sorted)
	for _, n := range sorted {
		activeSet[n] = true
	}

	var sb strings.Builder

	sb.WriteString("## Active Tools\n\n")
	sb.WriteString(strings.Join(sorted, ", "))
	sb.WriteString("\n")

	if compact {
		return sb.String()
	}

	var inactive []string
	for name := range allDescs {
		if !activeSet[name] {
			inactive = append(inactive, name)
		}
	}
	sort.Strings(inactive)

	if len(inactive) > 0 {
		sb.WriteString("\n## Additional Tools (call activate_tools to enable)\n\n")
		sb.WriteString(strings.Join(inactive, ", "))
		sb.WriteString("\n")
	}

	return sb.String()
}

// SessionSection builds the "## Session Context" block from primitive values.
func SessionSection(rootDir, language, title string, msgCount int) string {
	var sb strings.Builder
	sb.WriteString("## Session Context\n\n")
	hasContent := false

	if rootDir != "" {
		sb.WriteString(fmt.Sprintf("Working directory: %s\n", rootDir))
		hasContent = true
	}
	if language != "" {
		sb.WriteString(fmt.Sprintf("Preferred language: %s\n", language))
		hasContent = true
	}
	if title != "" && msgCount > 0 {
		sb.WriteString(fmt.Sprintf("Resumed session: %q.\n", title))
		hasContent = true
	}
	if msgCount > 0 {
		sb.WriteString(fmt.Sprintf("%d previous messages.", msgCount))
		hasContent = true
	}

	if !hasContent {
		return ""
	}
	return sb.String()
}

// SkillSection builds the "## Available Skills" block.
// When compact is true, at most 5 skills are shown.
func SkillSection(skills map[string]string, compact bool) string {
	if len(skills) == 0 {
		return ""
	}

	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, name)
	}
	sort.Strings(names)

	maxSkills := len(names)
	if compact && maxSkills > 5 {
		maxSkills = 5
	}

	var sb strings.Builder
	sb.WriteString("## Available Skills\n\n")
	sb.WriteString("Use `activate_skill(name)` to load a skill's full instructions when relevant.\n")
	sb.WriteString("Skills with a workflow can be executed via `run_workflow(skill_name, vars)`.\n\n")
	for _, name := range names[:maxSkills] {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", name, skills[name]))
	}
	sb.WriteString("\n")
	return sb.String()
}

// ActorSection builds the "## Available Actors" block.
func ActorSection(actors []ActorInfo) string {
	if len(actors) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Available Actors\n\n")
	sb.WriteString("You can delegate tasks to these specialized actors using submit_task or plan_task.\n")
	sb.WriteString("Use actor_tags to target a specific actor type.\n\n")
	for _, a := range actors {
		sb.WriteString(fmt.Sprintf("- **%s** — tags: %v, capabilities: %v\n",
			a.Name, a.Tags, a.Capabilities))
		if a.PromptPrefix != "" {
			prefix := a.PromptPrefix
			if len(prefix) > 80 {
				prefix = prefix[:80] + "..."
			}
			sb.WriteString(fmt.Sprintf("  Role: %s\n", prefix))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// MemorySection builds the "## Relevant Memories" block.
// If contentMax > 0, each memory's content is truncated to that length.
func MemorySection(memories []MemoryInfo, contentMax int) string {
	if len(memories) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Relevant Memories\n\n")
	for _, m := range memories {
		content := m.Content
		if contentMax > 0 && len(content) > contentMax {
			content = content[:contentMax] + "..."
		}
		sb.WriteString(fmt.Sprintf("- **[%s] %s**: %s\n", m.Type, m.Title, content))
	}
	return sb.String()
}

// LanguageSection builds the "## Language" block for a given language code.
func LanguageSection(langCode string) string {
	if langCode == "" {
		return ""
	}
	langName := preferredLanguageName(langCode)
	return fmt.Sprintf("## Language\n\nThe user prefers to be answered in %s. Always respond in %s unless explicitly asked otherwise.\n\n", langName, langName)
}

// CustomInstructionSection wraps custom instructions in a section header.
func CustomInstructionSection(instructions string) string {
	if instructions == "" {
		return ""
	}
	return "## Additional Instructions\n\n" + instructions + "\n\n"
}

// RuntimeSection builds the "## Runtime Environment" prompt section.
// Returns "" in local mode with no tools available.
func RuntimeSection(environment string, tools []SystemTool) string {
	if environment == "local" && len(tools) == 0 {
		return ""
	}

	var sb strings.Builder

	if environment == "container" {
		sb.WriteString("## Runtime Environment\n\n")
		sb.WriteString("You are running in **container** mode.\n")
		sb.WriteString("- Tasks involving build, dev, or testing should be isolated in dedicated containers when possible.\n")
		sb.WriteString("- Use `docker run` to launch ephemeral containers for language-specific tasks (e.g., node, python, go).\n")
		sb.WriteString("- You have access to the host Docker daemon via mounted socket.\n")
	}

	if len(tools) > 0 {
		if environment != "container" {
			sb.WriteString("## System Tools Available\n\n")
		} else {
			sb.WriteString("\n### System Tools Available\n")
		}
		for _, t := range tools {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", t.Name, t.Version))
		}
	}

	return sb.String()
}

// preferredLanguageName returns the display name for a language code.
func preferredLanguageName(code string) string {
	switch strings.ToLower(code) {
	case "fr":
		return "French"
	case "en":
		return "English"
	case "de":
		return "German"
	case "es":
		return "Spanish"
	case "it":
		return "Italian"
	case "pt":
		return "Portuguese"
	case "nl":
		return "Dutch"
	case "ja":
		return "Japanese"
	case "zh":
		return "Chinese"
	case "ko":
		return "Korean"
	default:
		return code
	}
}
