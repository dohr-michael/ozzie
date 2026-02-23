package plugins

import (
	"encoding/json"
	"regexp"
	"strings"
)

// destructiveRule describes a shell command pattern that should be blocked in autonomous mode.
type destructiveRule struct {
	pattern *regexp.Regexp
	reason  string
}

// destructivePatterns is the compiled denylist of dangerous shell patterns.
var destructivePatterns []destructiveRule

func init() {
	raw := []struct {
		pattern string
		reason  string
	}{
		// Filesystem destruction
		{`\brm\s+.*-[a-zA-Z]*[rR]`, "recursive remove"},
		{`\brm\s+.*-[a-zA-Z]*[fF]`, "force remove"},
		// Disk/partition
		{`\bdd\b\s+.*\bof=`, "raw disk write (dd)"},
		{`\bmkfs\b`, "filesystem format"},
		{`\bfdisk\b`, "partition edit"},
		// System
		{`:\(\)\s*\{`, "fork bomb"},
		{`>/dev/sd[a-z]`, "raw device write"},
		{`\bchmod\s+.*-[a-zA-Z]*[rR]`, "recursive chmod"},
		{`\bchown\s+.*-[a-zA-Z]*[rR]`, "recursive chown"},
		// Privilege escalation
		{`\bsudo\b`, "privilege escalation"},
		{`\bsu\s`, "switch user"},
	}
	destructivePatterns = make([]destructiveRule, len(raw))
	for i, r := range raw {
		destructivePatterns[i] = destructiveRule{
			pattern: regexp.MustCompile(r.pattern),
			reason:  r.reason,
		}
	}
}

// matchDestructivePattern checks a command string against the denylist.
// Returns the matched rule, or nil if the command is safe.
func matchDestructivePattern(command string) *destructiveRule {
	for i := range destructivePatterns {
		if destructivePatterns[i].pattern.MatchString(command) {
			return &destructivePatterns[i]
		}
	}
	return nil
}

// pathTokenPattern matches path-like tokens in shell commands:
// absolute paths (/...), home-relative (~/...), and relative with parent traversal (../).
var pathTokenPattern = regexp.MustCompile(`(?:^|\s)((?:/|~/|\.\./)[\w./_~-]*)`)

// extractCommandPaths extracts path-like tokens from a shell command string.
func extractCommandPaths(command string) []string {
	matches := pathTokenPattern.FindAllStringSubmatch(command, -1)
	if len(matches) == 0 {
		return nil
	}
	paths := make([]string, 0, len(matches))
	for _, m := range matches {
		p := strings.TrimSpace(m[1])
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// pathKeys are JSON field names that typically contain filesystem paths.
var pathKeys = map[string]bool{
	"path":        true,
	"working_dir": true,
}

// arrayPathKeys are JSON field names that typically contain arrays of filesystem paths.
var arrayPathKeys = map[string]bool{
	"paths": true,
}

// extractToolPaths extracts path-related fields from a tool's JSON arguments.
// It recursively searches for "path", "working_dir", and "paths" fields
// in nested objects (e.g. git tool's {"action":"status","args":{"path":"..."}}).
func extractToolPaths(argsJSON string) []string {
	var raw any
	if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
		return nil
	}
	var paths []string
	collectPaths(raw, &paths)
	return paths
}

// collectPaths recursively walks a JSON value and collects path strings.
func collectPaths(v any, paths *[]string) {
	switch val := v.(type) {
	case map[string]any:
		for key, child := range val {
			if pathKeys[key] {
				if s, ok := child.(string); ok && s != "" {
					*paths = append(*paths, s)
				}
			} else if arrayPathKeys[key] {
				if arr, ok := child.([]any); ok {
					for _, item := range arr {
						if s, ok := item.(string); ok && s != "" {
							*paths = append(*paths, s)
						}
					}
				}
			}
			// Recurse into nested objects/arrays
			collectPaths(child, paths)
		}
	case []any:
		for _, item := range val {
			collectPaths(item, paths)
		}
	}
}
