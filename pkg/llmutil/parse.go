package llmutil

import "strings"

// StripCodeFences removes markdown code fences (```...```) wrapping content.
// Returns the content between the first and last fence markers.
// If no fences are found, returns the input unchanged.
func StripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	// Remove opening fence line
	lines = lines[1:]
	// Remove closing fence line if present
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
