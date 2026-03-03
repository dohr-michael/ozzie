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
	mw := NewToolRecoveryMiddleware(ToolRecoveryConfig{})
	wrapped := mw.Invokable(failingEndpoint(errors.New("permission denied")))

	out, err := wrapped(context.Background(), &compose.ToolInput{Name: "read_file"})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !strings.Contains(out.Result, "[TOOL_ERROR]") {
		t.Fatalf("expected TOOL_ERROR marker, got: %s", out.Result)
	}
	if !strings.Contains(out.Result, "permission denied") {
		t.Fatalf("expected error text in result, got: %s", out.Result)
	}
}

func TestToolRecovery_NeverPropagatesErrors(t *testing.T) {
	mw := NewToolRecoveryMiddleware(ToolRecoveryConfig{})
	wrapped := mw.Invokable(failingEndpoint(errors.New("persistent failure")))
	input := &compose.ToolInput{Name: "search"}

	// Call many times — error should never be propagated
	for i := 0; i < 10; i++ {
		out, err := wrapped(context.Background(), input)
		if err != nil {
			t.Fatalf("attempt %d: expected recovery, got error: %v", i+1, err)
		}
		if !strings.Contains(out.Result, "[TOOL_ERROR]") {
			t.Fatalf("attempt %d: expected TOOL_ERROR marker, got: %s", i+1, out.Result)
		}
	}
}

func TestToolRecovery_ConcurrentSafe(t *testing.T) {
	mw := NewToolRecoveryMiddleware(ToolRecoveryConfig{})
	wrapped := mw.Invokable(failingEndpoint(errors.New("fail")))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("tool_%d", n%5)
			out, err := wrapped(context.Background(), &compose.ToolInput{Name: name})
			if err != nil {
				t.Errorf("tool %s: unexpected error: %v", name, err)
			}
			if out == nil || !strings.Contains(out.Result, "[TOOL_ERROR]") {
				t.Errorf("tool %s: expected TOOL_ERROR marker", name)
			}
		}(i)
	}
	wg.Wait()
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

func TestToolRecovery_ErrorMessageContainsToolName(t *testing.T) {
	mw := NewToolRecoveryMiddleware(ToolRecoveryConfig{})
	wrapped := mw.Invokable(failingEndpoint(errors.New("file not found")))

	out, err := wrapped(context.Background(), &compose.ToolInput{Name: "read_file"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.Result, `"read_file"`) {
		t.Fatalf("expected tool name in error message, got: %s", out.Result)
	}
}
