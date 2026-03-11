package events

import "context"

type sessionIDKey struct{}
type autonomousKey struct{}

// ContextWithSessionID returns a new context carrying the session ID.
func ContextWithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, id)
}

// SessionIDFromContext extracts the session ID from the context, or "" if absent.
func SessionIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(sessionIDKey{}).(string); ok {
		return id
	}
	return ""
}

// WithAutonomous marks the context as running in autonomous mode (async tasks).
// In this mode, dangerous tools that aren't pre-approved will fail immediately
// instead of waiting for interactive confirmation.
func WithAutonomous(ctx context.Context) context.Context {
	return context.WithValue(ctx, autonomousKey{}, true)
}

// IsAutonomousContext returns true if the context is in autonomous mode.
func IsAutonomousContext(ctx context.Context) bool {
	v, _ := ctx.Value(autonomousKey{}).(bool)
	return v
}

type workDirKey struct{}
type taskEnvKey struct{}

// ContextWithWorkDir returns a new context carrying the working directory.
func ContextWithWorkDir(ctx context.Context, dir string) context.Context {
	if dir == "" {
		return ctx
	}
	return context.WithValue(ctx, workDirKey{}, dir)
}

// WorkDirFromContext extracts the working directory from the context, or "" if absent.
func WorkDirFromContext(ctx context.Context) string {
	dir, _ := ctx.Value(workDirKey{}).(string)
	return dir
}

// ContextWithTaskEnv returns a new context carrying task environment variables.
func ContextWithTaskEnv(ctx context.Context, env map[string]string) context.Context {
	if len(env) == 0 {
		return ctx
	}
	return context.WithValue(ctx, taskEnvKey{}, env)
}

// TaskEnvFromContext extracts task environment variables from the context, or nil if absent.
func TaskEnvFromContext(ctx context.Context) map[string]string {
	env, _ := ctx.Value(taskEnvKey{}).(map[string]string)
	return env
}

type toolConstraintsKey struct{}

// ToolConstraint defines argument-level restrictions for a specific tool.
// Constraints are dynamic (per-session, per-task) and carried in the Go context.
type ToolConstraint struct {
	AllowedCommands []string `json:"allowed_commands,omitempty"` // binary allowlist (run_command)
	AllowedPatterns []string `json:"allowed_patterns,omitempty"` // regex on full command string
	BlockedPatterns []string `json:"blocked_patterns,omitempty"` // regex denylist (additive)
	AllowedPaths    []string `json:"allowed_paths,omitempty"`    // glob allowlist for paths
	AllowedDomains  []string `json:"allowed_domains,omitempty"`  // domain allowlist (web_fetch)
}

// ContextWithToolConstraints returns a context carrying per-tool constraints.
func ContextWithToolConstraints(ctx context.Context, constraints map[string]*ToolConstraint) context.Context {
	if len(constraints) == 0 {
		return ctx
	}
	return context.WithValue(ctx, toolConstraintsKey{}, constraints)
}

// ToolConstraintsFromContext extracts tool constraints from the context, or nil if absent.
func ToolConstraintsFromContext(ctx context.Context) map[string]*ToolConstraint {
	c, _ := ctx.Value(toolConstraintsKey{}).(map[string]*ToolConstraint)
	return c
}

// MergeToolConstraints returns the intersection (most restrictive) of two constraint maps.
// Task-specific constraints override session constraints for the same tool.
// Nil constraints are treated as "no restriction" (pass-through).
func MergeToolConstraints(session, task map[string]*ToolConstraint) map[string]*ToolConstraint {
	if len(session) == 0 {
		return task
	}
	if len(task) == 0 {
		return session
	}
	merged := make(map[string]*ToolConstraint, len(session)+len(task))
	for k, v := range session {
		merged[k] = v
	}
	for k, v := range task {
		if existing, ok := merged[k]; ok {
			merged[k] = mergeConstraint(existing, v)
		} else {
			merged[k] = v
		}
	}
	return merged
}

// mergeConstraint merges two constraints for the same tool (intersection — most restrictive wins).
func mergeConstraint(a, b *ToolConstraint) *ToolConstraint {
	m := &ToolConstraint{}
	// For allowlists: if both set, keep the shorter (more restrictive).
	// If only one set, keep it (it's a restriction vs. none).
	m.AllowedCommands = intersectOrKeep(a.AllowedCommands, b.AllowedCommands)
	m.AllowedPatterns = mergeSlice(a.AllowedPatterns, b.AllowedPatterns)
	m.BlockedPatterns = mergeSlice(a.BlockedPatterns, b.BlockedPatterns)
	m.AllowedPaths = intersectOrKeep(a.AllowedPaths, b.AllowedPaths)
	m.AllowedDomains = intersectOrKeep(a.AllowedDomains, b.AllowedDomains)
	return m
}

// intersectOrKeep returns the intersection if both are set, or whichever is set.
func intersectOrKeep(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	set := make(map[string]bool, len(b))
	for _, v := range b {
		set[v] = true
	}
	var result []string
	for _, v := range a {
		if set[v] {
			result = append(result, v)
		}
	}
	return result
}

// mergeSlice concatenates two slices (additive for deny/pattern lists).
func mergeSlice(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	result := make([]string, 0, len(a)+len(b))
	result = append(result, a...)
	result = append(result, b...)
	return result
}

type policyNameKey struct{}

// ContextWithPolicyName returns a context carrying the active policy name.
func ContextWithPolicyName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, policyNameKey{}, name)
}

// PolicyNameFromContext extracts the policy name from the context, or "" if absent.
func PolicyNameFromContext(ctx context.Context) string {
	name, _ := ctx.Value(policyNameKey{}).(string)
	return name
}

type taskIDKey struct{}

// ContextWithTaskID returns a context carrying the current task ID.
func ContextWithTaskID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, taskIDKey{}, id)
}

// TaskIDFromContext extracts the task ID from the context, or "" if absent.
func TaskIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(taskIDKey{}).(string)
	return id
}
