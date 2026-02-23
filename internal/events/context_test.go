package events

import (
	"context"
	"testing"
)

func TestSessionIDRoundTrip(t *testing.T) {
	ctx := ContextWithSessionID(context.Background(), "sess_abc123")
	got := SessionIDFromContext(ctx)
	if got != "sess_abc123" {
		t.Errorf("got %q, want %q", got, "sess_abc123")
	}
}

func TestSessionIDFromEmptyContext(t *testing.T) {
	got := SessionIDFromContext(context.Background())
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestWorkDirRoundTrip(t *testing.T) {
	ctx := ContextWithWorkDir(context.Background(), "/home/user/project")
	got := WorkDirFromContext(ctx)
	if got != "/home/user/project" {
		t.Errorf("got %q, want %q", got, "/home/user/project")
	}
}

func TestWorkDirFromEmptyContext(t *testing.T) {
	got := WorkDirFromContext(context.Background())
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestWorkDirEmptyStringNoOp(t *testing.T) {
	bg := context.Background()
	ctx := ContextWithWorkDir(bg, "")
	if ctx != bg {
		t.Error("expected same context when dir is empty")
	}
}

func TestTaskEnvRoundTrip(t *testing.T) {
	env := map[string]string{"PROJECT_NAME": "chess", "LANGUAGE": "go"}
	ctx := ContextWithTaskEnv(context.Background(), env)
	got := TaskEnvFromContext(ctx)
	if len(got) != 2 || got["PROJECT_NAME"] != "chess" || got["LANGUAGE"] != "go" {
		t.Errorf("got %v, want %v", got, env)
	}
}

func TestTaskEnvFromEmptyContext(t *testing.T) {
	got := TaskEnvFromContext(context.Background())
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestTaskEnvEmptyMapNoOp(t *testing.T) {
	bg := context.Background()
	ctx := ContextWithTaskEnv(bg, nil)
	if ctx != bg {
		t.Error("expected same context when env is nil")
	}
	ctx = ContextWithTaskEnv(bg, map[string]string{})
	if ctx != bg {
		t.Error("expected same context when env is empty")
	}
}
