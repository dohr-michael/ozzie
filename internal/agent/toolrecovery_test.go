package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/compose"
)

// fakeEndpoint returns a canned result.
func fakeEndpoint(result string) compose.InvokableToolEndpoint {
	return func(_ context.Context, _ *compose.ToolInput) (*compose.ToolOutput, error) {
		return &compose.ToolOutput{Result: result}, nil
	}
}

// failingEndpoint always returns the given error.
func failingEndpoint(err error) compose.InvokableToolEndpoint {
	return func(_ context.Context, _ *compose.ToolInput) (*compose.ToolOutput, error) {
		return nil, err
	}
}

func TestToolRecovery_PassesSuccessThrough(t *testing.T) {
	mw := NewToolRecoveryMiddleware(ToolRecoveryConfig{})
	wrapped := mw.Invokable(fakeEndpoint("ok"))

	out, err := wrapped(context.Background(), &compose.ToolInput{Name: "read_file"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Result != "ok" {
		t.Fatalf("expected result %q, got %q", "ok", out.Result)
	}
}

func TestToolRecovery_ConvertsErrorToResult(t *testing.T) {
	mw := NewToolRecoveryMiddleware(ToolRecoveryConfig{MaxRetries: 3})
	wrapped := mw.Invokable(failingEndpoint(errors.New("permission denied")))

	out, err := wrapped(context.Background(), &compose.ToolInput{Name: "read_file"})
	if err != nil {
		t.Fatalf("expected nil error on first attempt, got: %v", err)
	}
	if !strings.Contains(out.Result, "[TOOL_ERROR]") {
		t.Fatalf("expected TOOL_ERROR marker, got: %s", out.Result)
	}
	if !strings.Contains(out.Result, "attempt 1/3") {
		t.Fatalf("expected attempt 1/3, got: %s", out.Result)
	}
	if !strings.Contains(out.Result, "permission denied") {
		t.Fatalf("expected error text in result, got: %s", out.Result)
	}
}

func TestToolRecovery_PropagatesAfterMaxRetries(t *testing.T) {
	mw := NewToolRecoveryMiddleware(ToolRecoveryConfig{MaxRetries: 2})
	origErr := errors.New("not found")
	wrapped := mw.Invokable(failingEndpoint(origErr))
	input := &compose.ToolInput{Name: "search"}

	// First call: recovered
	out, err := wrapped(context.Background(), input)
	if err != nil {
		t.Fatalf("attempt 1: expected recovery, got error: %v", err)
	}
	if !strings.Contains(out.Result, "[TOOL_ERROR]") {
		t.Fatalf("attempt 1: expected TOOL_ERROR marker")
	}

	// Second call: max reached, error propagated
	_, err = wrapped(context.Background(), input)
	if err == nil {
		t.Fatal("attempt 2: expected propagated error, got nil")
	}
	if !errors.Is(err, origErr) {
		t.Fatalf("attempt 2: expected original error, got: %v", err)
	}
}

func TestToolRecovery_TracksPerToolName(t *testing.T) {
	mw := NewToolRecoveryMiddleware(ToolRecoveryConfig{MaxRetries: 2})
	wrapped := mw.Invokable(failingEndpoint(errors.New("fail")))

	// Tool A: first call recovered, second exhausts budget
	out, err := wrapped(context.Background(), &compose.ToolInput{Name: "tool_a"})
	if err != nil {
		t.Fatalf("tool_a attempt 1: expected recovery, got error: %v", err)
	}
	if !strings.Contains(out.Result, "[TOOL_ERROR]") {
		t.Fatal("tool_a attempt 1: expected TOOL_ERROR marker")
	}

	_, err = wrapped(context.Background(), &compose.ToolInput{Name: "tool_a"})
	if err == nil {
		t.Fatal("tool_a attempt 2: expected propagated error")
	}

	// Tool B: independent counter, first call should be recovered
	out, err = wrapped(context.Background(), &compose.ToolInput{Name: "tool_b"})
	if err != nil {
		t.Fatalf("tool_b: expected recovery, got error: %v", err)
	}
	if !strings.Contains(out.Result, "[TOOL_ERROR]") {
		t.Fatal("tool_b: expected TOOL_ERROR marker")
	}
}

func TestToolRecovery_DefaultMaxRetries(t *testing.T) {
	mw := NewToolRecoveryMiddleware(ToolRecoveryConfig{}) // zero-value
	wrapped := mw.Invokable(failingEndpoint(errors.New("fail")))
	input := &compose.ToolInput{Name: "test"}

	// Should recover DefaultMaxToolRetries-1 times, then propagate
	for i := 0; i < DefaultMaxToolRetries-1; i++ {
		out, err := wrapped(context.Background(), input)
		if err != nil {
			t.Fatalf("attempt %d: expected recovery, got error: %v", i+1, err)
		}
		if !strings.Contains(out.Result, fmt.Sprintf("attempt %d/%d", i+1, DefaultMaxToolRetries)) {
			t.Fatalf("attempt %d: wrong attempt number in result: %s", i+1, out.Result)
		}
	}

	// Next call should propagate
	_, err := wrapped(context.Background(), input)
	if err == nil {
		t.Fatal("expected propagated error after max retries")
	}
}

func TestToolRecovery_ConcurrentSafe(t *testing.T) {
	mw := NewToolRecoveryMiddleware(ToolRecoveryConfig{MaxRetries: 100})
	wrapped := mw.Invokable(failingEndpoint(errors.New("fail")))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("tool_%d", n%5)
			_, _ = wrapped(context.Background(), &compose.ToolInput{Name: name})
		}(i)
	}
	wg.Wait()
	// No panic = pass
}

func TestToolRecovery_EmptyResultGuard(t *testing.T) {
	mw := NewToolRecoveryMiddleware(ToolRecoveryConfig{})
	// Tool returns empty string — should be replaced with "[OK]"
	wrapped := mw.Invokable(fakeEndpoint(""))

	out, err := wrapped(context.Background(), &compose.ToolInput{Name: "write_file"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Result != "[OK]" {
		t.Fatalf("expected [OK] for empty result, got %q", out.Result)
	}
}

func TestFormatToolError(t *testing.T) {
	msg := formatToolError("read_file", 1, 3, errors.New("permission denied: /etc/shadow"))
	expected := `[TOOL_ERROR] Tool "read_file" failed (attempt 1/3): permission denied: /etc/shadow
You can retry with different parameters, or inform the user about the issue.`
	if msg != expected {
		t.Fatalf("unexpected message:\ngot:  %s\nwant: %s", msg, expected)
	}
}
