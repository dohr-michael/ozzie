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
