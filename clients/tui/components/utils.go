// Package components provides reusable TUI components.
package components

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TruncateString truncates a string to maxLen characters, adding "..." if truncated.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// WrapText wraps text to the specified width.
func WrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// Handle empty lines
		if line == "" {
			continue
		}

		// Simple word wrap
		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		currentLine := words[0]
		for _, word := range words[1:] {
			if len(currentLine)+1+len(word) <= width {
				currentLine += " " + word
			} else {
				result.WriteString(currentLine)
				result.WriteString("\n")
				currentLine = word
			}
		}
		result.WriteString(currentLine)
	}

	return result.String()
}

// FormatArguments formats tool arguments as a compact JSON string.
func FormatArguments(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}

	// Build compact representation
	var parts []string
	for k, v := range args {
		var valStr string
		switch val := v.(type) {
		case string:
			valStr = TruncateString(val, 50)
		case []any:
			valStr = fmt.Sprintf("[%d items]", len(val))
		case map[string]any:
			valStr = fmt.Sprintf("{%d keys}", len(val))
		default:
			valStr = fmt.Sprintf("%v", val)
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, valStr))
	}

	return strings.Join(parts, ", ")
}

// FormatJSON formats a value as indented JSON.
func FormatJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

// FormatTokenCount formats a number with space as thousand separator.
func FormatTokenCount(n int) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	var result strings.Builder
	offset := len(str) % 3
	if offset > 0 {
		result.WriteString(str[:offset])
	}
	for i := offset; i < len(str); i += 3 {
		if result.Len() > 0 {
			result.WriteString(" ")
		}
		result.WriteString(str[i : i+3])
	}
	return result.String()
}

// PadRight pads a string to the right with spaces.
func PadRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// PadLeft pads a string to the left with spaces.
func PadLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}
