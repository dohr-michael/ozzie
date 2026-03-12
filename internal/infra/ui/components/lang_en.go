package components

import "github.com/dohr-michael/ozzie/internal/infra/i18n"

func init() {
	i18n.Register("en", map[string]string{
		// Shared hints
		"hint.quit":        "ctrl+c=quit",
		"hint.back":        "esc=back",
		"hint.submit_back": "enter=submit • esc=back",

		// Confirm
		"confirm.yes": "Yes",
		"confirm.no":  "No",

		// Input
		"input.placeholder.chat":  "Type a message...",
		"input.placeholder.value": "Enter value...",
		"input.error.required":    "This field is required",
		"input.error.invalid":     "Invalid format",
		"input.waiting":           "  Waiting for response...",

		// Hints
		"hint.text":    "enter=submit • esc=cancel",
		"hint.select":  "↑↓=navigate • enter=select • esc=cancel",
		"hint.multi":   "↑↓=navigate • space=toggle • enter=submit • esc=cancel",
		"hint.confirm": "y/n or ↑↓ + enter • esc=cancel",
		"hint.scroll":  "↑↓=scroll",

		// Chat
		"chat.thinking":        "Thinking...",
		"chat.welcome.tagline": " — Your personal AI agent operating system.",
		"chat.tips.quit":       "  Tips: Ctrl+C to quit",
		"chat.tool.no_output":  "(No output)",
		"chat.tool.more_lines": "... (%d more lines)",
		"chat.tool.awaiting":   " (awaiting confirmation)",
		"chat.tool.denied":     " (denied)",

		// Header
		"header.tokens":    " tokens",
		"header.streaming": "● streaming",

		// Roles
		"role.system": "System: ",
	})
}
