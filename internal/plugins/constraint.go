package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/events"
)

// ConstraintGuard wraps a tool.InvokableTool with argument-level validation.
// Constraints are read from the Go context at each invocation — if no constraints
// are present for this tool, the call passes through.
type ConstraintGuard struct {
	inner    tool.InvokableTool
	toolName string
}

// WrapConstraint wraps a single tool with constraint validation.
func WrapConstraint(t tool.InvokableTool, name string) tool.InvokableTool {
	return &ConstraintGuard{inner: t, toolName: name}
}

// WrapRegistryConstraints wraps all tools in the registry with constraint validation.
func WrapRegistryConstraints(registry *ToolRegistry) {
	for _, name := range registry.ToolNames() {
		registry.tools[name] = WrapConstraint(registry.tools[name], name)
	}
}

// Info delegates to the inner tool.
func (g *ConstraintGuard) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return g.inner.Info(ctx)
}

// InvokableRun validates arguments against constraints before delegating.
func (g *ConstraintGuard) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	constraints := events.ToolConstraintsFromContext(ctx)
	if constraints == nil {
		return g.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	}

	tc, ok := constraints[g.toolName]
	if !ok || tc == nil {
		return g.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	}

	if err := g.validate(tc, argumentsInJSON); err != nil {
		return "", fmt.Errorf("constraint: %s: %w", g.toolName, err)
	}

	return g.inner.InvokableRun(ctx, argumentsInJSON, opts...)
}

// validate dispatches validation based on which constraint fields are set.
func (g *ConstraintGuard) validate(tc *events.ToolConstraint, argsJSON string) error {
	// Extract command string (for run_command-style tools)
	var args struct {
		Command string `json:"command"`
		URL     string `json:"url"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &args)

	// Subshell detection: when command restrictions are active, block subshell
	// syntax that could bypass them ($(...), backticks, <(...)).
	if (len(tc.AllowedCommands) > 0 || len(tc.AllowedPatterns) > 0) && args.Command != "" {
		if containsSubshellAST(args.Command) {
			return fmt.Errorf("subshell/process substitution not allowed with command restrictions")
		}
	}

	// AllowedCommands: check ALL binaries in chained commands (AST-based)
	if len(tc.AllowedCommands) > 0 && args.Command != "" {
		for _, binary := range extractAllBinariesAST(args.Command) {
			if !slices.Contains(tc.AllowedCommands, binary) {
				return fmt.Errorf("command %q not in allowed list %v", binary, tc.AllowedCommands)
			}
		}
	}

	// AllowedPatterns: each sub-command must match at least one pattern
	if len(tc.AllowedPatterns) > 0 && args.Command != "" {
		for _, sub := range splitSubCommands(args.Command) {
			if !matchesAnyPattern(sub, tc.AllowedPatterns) {
				return fmt.Errorf("command %q does not match any allowed pattern", sub)
			}
		}
	}

	// BlockedPatterns: check each sub-command against denylist
	if len(tc.BlockedPatterns) > 0 && args.Command != "" {
		for _, sub := range splitSubCommands(args.Command) {
			if pattern, matched := matchesPattern(sub, tc.BlockedPatterns); matched {
				return fmt.Errorf("command %q matches blocked pattern %q", sub, pattern)
			}
		}
	}

	// AllowedPaths: glob match on extracted paths (AST + JSON)
	if len(tc.AllowedPaths) > 0 {
		var paths []string
		if args.Command != "" {
			paths = append(paths, extractCommandPathsAST(args.Command)...)
		}
		paths = append(paths, extractToolPaths(argsJSON)...)
		for _, p := range paths {
			if !matchesAnyGlob(p, tc.AllowedPaths) {
				return fmt.Errorf("path %q not in allowed paths %v", p, tc.AllowedPaths)
			}
		}
	}

	// AllowedDomains: domain match on URL
	if len(tc.AllowedDomains) > 0 && args.URL != "" {
		if !matchesDomain(args.URL, tc.AllowedDomains) {
			return fmt.Errorf("domain of %q not in allowed domains %v", args.URL, tc.AllowedDomains)
		}
	}

	return nil
}

// shellChainSplitter splits a command on shell chaining operators (&&, ||, ;, |).
// It returns the individual sub-command strings.
var shellChainSplitter = regexp.MustCompile(`\s*(?:&&|\|\||\||;)\s*`)

// extractBinary returns the first token (binary name) from a command string.
// Handles env prefixes like "VAR=val cmd" and leading whitespace.
func extractBinary(command string) string {
	command = strings.TrimSpace(command)
	// Skip env-style prefixes (KEY=VALUE)
	for {
		if idx := strings.IndexByte(command, '='); idx > 0 {
			spaceIdx := strings.IndexByte(command, ' ')
			if spaceIdx > idx {
				command = strings.TrimSpace(command[spaceIdx+1:])
				continue
			}
		}
		break
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	// Return just the binary name without path
	return filepath.Base(fields[0])
}

// extractAllBinariesRegex splits a command on shell chaining operators (&&, ||, ;, |)
// and returns the binary name from each sub-command.
// Deprecated: use extractAllBinariesAST instead for better accuracy.
func extractAllBinariesRegex(command string) []string {
	segments := shellChainSplitter.Split(command, -1)
	var binaries []string
	for _, seg := range segments {
		if b := extractBinary(seg); b != "" {
			binaries = append(binaries, b)
		}
	}
	return binaries
}

// splitSubCommands splits a command on shell chaining operators and returns
// the trimmed sub-command strings.
func splitSubCommands(command string) []string {
	segments := shellChainSplitter.Split(command, -1)
	var result []string
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg != "" {
			result = append(result, seg)
		}
	}
	return result
}

// matchesAnyPattern returns true if s matches any regex in patterns.
func matchesAnyPattern(s string, patterns []string) bool {
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// matchesPattern returns the first matching pattern and true, or ("", false).
func matchesPattern(s string, patterns []string) (string, bool) {
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		if re.MatchString(s) {
			return p, true
		}
	}
	return "", false
}

// matchesAnyGlob checks if path matches any of the glob patterns.
func matchesAnyGlob(path string, globs []string) bool {
	clean := filepath.Clean(path)
	for _, g := range globs {
		if matched, _ := filepath.Match(g, clean); matched {
			return true
		}
		// Also check if the path is under the glob prefix (for directory patterns)
		gClean := filepath.Clean(g)
		if strings.HasSuffix(gClean, "*") {
			prefix := strings.TrimSuffix(gClean, "*")
			if strings.HasPrefix(clean, prefix) {
				return true
			}
		}
	}
	return false
}

// matchesDomain checks if the URL's host matches any allowed domain.
// Supports wildcard prefixes like "*.example.com".
func matchesDomain(rawURL string, domains []string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "" {
		return false
	}

	for _, d := range domains {
		if strings.HasPrefix(d, "*.") {
			suffix := d[1:] // ".example.com"
			if strings.HasSuffix(host, suffix) || host == d[2:] {
				return true
			}
		} else if host == d {
			return true
		}
	}
	return false
}

var _ tool.InvokableTool = (*ConstraintGuard)(nil)
