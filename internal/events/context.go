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

// ValidationRequest is sent by the request_validation tool to signal self-suspension.
type ValidationRequest struct {
	Token   string
	Content string
}

type validationChKey struct{}

// ContextWithValidationCh returns a context carrying a validation request channel.
func ContextWithValidationCh(ctx context.Context, ch chan<- ValidationRequest) context.Context {
	return context.WithValue(ctx, validationChKey{}, ch)
}

// ValidationChFromContext extracts the validation channel from the context, or nil if absent.
func ValidationChFromContext(ctx context.Context) chan<- ValidationRequest {
	ch, _ := ctx.Value(validationChKey{}).(chan<- ValidationRequest)
	return ch
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
