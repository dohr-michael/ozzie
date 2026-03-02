// Package components provides reusable TUI components.
package components

import (
	"fmt"
	"strings"
)

// ToolCallStatus represents the state of a tool call.
type ToolCallStatus int

const (
	ToolStatusPending ToolCallStatus = iota
	ToolStatusRunning
	ToolStatusAwaitingConfirmation
	ToolStatusConfirmed
	ToolStatusDenied
	ToolStatusCompleted
	ToolStatusFailed
)

// ToolCall represents a tool invocation with its result.
type ToolCall struct {
	Name      string
	Arguments string
	Result    string
	Error     error
	Status    ToolCallStatus
	Completed bool
}

// ---------------------------------------------------------------------------
// Package-level render helpers (used by App for tea.Println flushing)
// ---------------------------------------------------------------------------

// RenderUserMessage renders a user message with ❯ prefix.
func RenderUserMessage(content string, width int) string {
	prefix := InputPromptCharStyle.Render("❯ ")
	wrapped := wrapText(content, width-4)
	lines := strings.Split(wrapped, "\n")

	var result strings.Builder
	for i, line := range lines {
		if i == 0 {
			result.WriteString(prefix + UserStyle.Render(line))
		} else {
			result.WriteString("\n  " + UserStyle.Render(line))
		}
	}
	return result.String()
}

// RenderToolResult renders a single completed tool call.
func RenderToolResult(tool ToolCall, width int) string {
	return renderSingleTool(tool, width)
}

// RenderExpandedTools renders a list of tool calls in Claude Code style.
func RenderExpandedTools(tools []ToolCall, width int) string {
	var b strings.Builder
	for i, tool := range tools {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(renderSingleTool(tool, width))
	}
	return b.String()
}

// RenderThinking renders the thinking indicator.
func RenderThinking() string {
	return ToolBulletStyle.Render("⏺ ") + ThinkingStyle.Render("Thinking...")
}

// RenderWelcome renders the welcome message.
func RenderWelcome() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(WelcomeTitleStyle.Render("Ozzie"))
	b.WriteString(WelcomeSubtitleStyle.Render(" — Your personal AI agent operating system."))
	b.WriteString("\n\n")
	b.WriteString(HelpTextStyle.Render("  Tips: Ctrl+C to quit"))
	b.WriteString("\n")
	return b.String()
}

// RenderError renders an error message.
func RenderError(content string, width int) string {
	wrapped := wrapText(content, width-2)
	return ErrorStyle.Render(wrapped)
}

// RenderAssistantMessage renders an assistant message with markdown formatting.
func RenderAssistantMessage(content string, width int) string {
	return RenderMarkdownWithWidth(content, width-2)
}

// RenderStreamingText renders in-progress streaming text with a cursor.
func RenderStreamingText(content string) string {
	return content + SpinnerStyle.Render("▌")
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// renderSingleTool renders one tool call entry.
func renderSingleTool(tool ToolCall, width int) string {
	var b strings.Builder

	// Bullet color depends on status
	var bullet string
	switch tool.Status {
	case ToolStatusAwaitingConfirmation:
		bullet = ConfirmWaitStyle.Render("⏺ ")
	case ToolStatusConfirmed:
		bullet = ConfirmApprovedStyle.Render("⏺ ")
	case ToolStatusDenied:
		bullet = ConfirmDeniedStyle.Render("⏺ ")
	case ToolStatusFailed:
		bullet = ToolErrorStyle.Render("⏺ ")
	case ToolStatusCompleted:
		bullet = ToolBulletStyle.Render("⏺ ")
	default:
		if tool.Completed && tool.Error != nil {
			bullet = ToolErrorStyle.Render("⏺ ")
		} else {
			bullet = ToolBulletStyle.Render("⏺ ")
		}
	}

	// Name(args) or Name ...
	name := ToolNameStyle.Render(tool.Name)
	if tool.Arguments != "" {
		args := TruncateString(tool.Arguments, 60)
		b.WriteString(bullet + name + ToolArgsStyle.Render("("+args+")"))
	} else {
		b.WriteString(bullet + name)
	}

	// Running indicator
	if !tool.Completed && tool.Status != ToolStatusAwaitingConfirmation &&
		tool.Status != ToolStatusConfirmed && tool.Status != ToolStatusDenied {
		b.WriteString(ToolArgsStyle.Render(" ..."))
	}

	// Status suffix for special states
	switch tool.Status {
	case ToolStatusAwaitingConfirmation:
		b.WriteString(ConfirmWaitStyle.Render(" (awaiting confirmation)"))
	case ToolStatusDenied:
		b.WriteString(ConfirmDeniedStyle.Render(" (denied)"))
	}

	// Result lines with ⎿ prefix
	if tool.Completed && tool.Error == nil {
		resultPrefix := ToolResultPrefixStyle.Render("  ⎿  ")
		if tool.Result == "" {
			b.WriteString("\n" + resultPrefix + ToolResultStyle.Render("(No output)"))
		} else {
			result := wrapText(tool.Result, width-6)
			lines := strings.Split(result, "\n")
			maxLines := 10
			for j, line := range lines {
				if j >= maxLines {
					b.WriteString("\n" + resultPrefix + ToolResultStyle.Render(fmt.Sprintf("... (%d more lines)", len(lines)-maxLines)))
					break
				}
				b.WriteString("\n" + resultPrefix + ToolResultStyle.Render(line))
			}
		}
	}

	// Error with ⎿ prefix
	if tool.Error != nil {
		resultPrefix := ToolResultPrefixStyle.Render("  ⎿  ")
		b.WriteString("\n" + resultPrefix + ToolErrorStyle.Render(tool.Error.Error()))
	}

	return b.String()
}

// wrapText wraps text to the specified width.
func wrapText(text string, width int) string {
	if width <= 0 {
		width = 80
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		if line == "" {
			continue
		}

		for len(line) > width {
			breakPoint := width
			for j := width; j > 0; j-- {
				if line[j] == ' ' {
					breakPoint = j
					break
				}
			}
			result.WriteString(line[:breakPoint])
			result.WriteString("\n")
			line = strings.TrimLeft(line[breakPoint:], " ")
		}
		result.WriteString(line)
	}

	return result.String()
}
