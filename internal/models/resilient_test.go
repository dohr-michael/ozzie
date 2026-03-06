package models

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/config"
)

// mockModel is a test double for model.ToolCallingChatModel.
type mockModel struct {
	generateFunc func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error)
	streamFunc   func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error)
}

func (m *mockModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, input, opts...)
	}
	return &schema.Message{Role: schema.Assistant, Content: "ok"}, nil
}

func (m *mockModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, input, opts...)
	}
	return nil, nil
}

func (m *mockModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// --- ResilientModel tests ---

func TestResilientModel_SuccessOnFirstAttempt(t *testing.T) {
	inner := &mockModel{}
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 5})
	rm := NewResilientModel(inner, cb, "test", nil)

	msg, err := rm.Generate(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Content != "ok" {
		t.Fatalf("expected 'ok', got %q", msg.Content)
	}
}

func TestResilientModel_RetryOnTimeout(t *testing.T) {
	var calls atomic.Int32
	inner := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			if calls.Add(1) == 1 {
				return nil, fmt.Errorf("context deadline exceeded")
			}
			return &schema.Message{Role: schema.Assistant, Content: "recovered"}, nil
		},
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 5})
	rm := NewResilientModel(inner, cb, "test", &config.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: config.Duration(time.Millisecond),
		MaxDelay:     config.Duration(10 * time.Millisecond),
		Multiplier:   1.0,
	})

	msg, err := rm.Generate(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if msg.Content != "recovered" {
		t.Fatalf("expected 'recovered', got %q", msg.Content)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls, got %d", calls.Load())
	}
}

func TestResilientModel_RetriesExhausted(t *testing.T) {
	inner := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 10})
	rm := NewResilientModel(inner, cb, "test", &config.RetryConfig{
		MaxAttempts:  2,
		InitialDelay: config.Duration(time.Millisecond),
		Multiplier:   1.0,
	})

	_, err := rm.Generate(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if !strings.Contains(err.Error(), "retries exhausted") {
		t.Fatalf("expected 'retries exhausted', got: %v", err)
	}
}

func TestResilientModel_NoRetryOnAuthError(t *testing.T) {
	var calls atomic.Int32
	inner := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			calls.Add(1)
			return nil, fmt.Errorf("401 unauthorized")
		},
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 5})
	rm := NewResilientModel(inner, cb, "test", &config.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: config.Duration(time.Millisecond),
	})

	_, err := rm.Generate(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 call (no retry on auth error), got %d", calls.Load())
	}
}

func TestResilientModel_CircuitBreakerOpens(t *testing.T) {
	inner := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			return nil, fmt.Errorf("connection timeout")
		},
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 2, Cooldown: time.Hour})
	rm := NewResilientModel(inner, cb, "test", &config.RetryConfig{
		MaxAttempts:  1, // no retry — each call counts as one failure
		InitialDelay: config.Duration(time.Millisecond),
	})

	// First call: fail → CB records failure (count=1)
	rm.Generate(context.Background(), nil)
	// Second call: fail → CB records failure (count=2) → CB opens
	rm.Generate(context.Background(), nil)

	// Third call: circuit is open → immediate ErrCircuitOpen
	_, err := rm.Generate(context.Background(), nil)
	if err == nil || !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got: %v", err)
	}
}

func TestResilientModel_CircuitBreakerRecovery(t *testing.T) {
	var calls atomic.Int32
	inner := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			if calls.Add(1) <= 2 {
				return nil, fmt.Errorf("connection timeout")
			}
			return &schema.Message{Role: schema.Assistant, Content: "recovered"}, nil
		},
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 2, Cooldown: 10 * time.Millisecond})
	rm := NewResilientModel(inner, cb, "test", &config.RetryConfig{
		MaxAttempts:  1,
		InitialDelay: config.Duration(time.Millisecond),
	})

	// Two failures → circuit opens
	rm.Generate(context.Background(), nil)
	rm.Generate(context.Background(), nil)

	// Wait for cooldown
	time.Sleep(15 * time.Millisecond)

	// Half-open: probe should succeed
	msg, err := rm.Generate(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected recovery, got: %v", err)
	}
	if msg.Content != "recovered" {
		t.Fatalf("expected 'recovered', got %q", msg.Content)
	}
	if cb.State() != cbClosed {
		t.Fatal("expected circuit to be closed after recovery")
	}
}

func TestResilientModel_StreamRetryBeforeFirstChunk(t *testing.T) {
	var calls atomic.Int32
	inner := &mockModel{
		streamFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
			if calls.Add(1) == 1 {
				return nil, fmt.Errorf("connection reset")
			}
			sr, sw := schema.Pipe[*schema.Message](1)
			go func() {
				sw.Send(&schema.Message{Content: "stream ok"}, nil)
				sw.Close()
			}()
			return sr, nil
		},
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 5})
	rm := NewResilientModel(inner, cb, "test", &config.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: config.Duration(time.Millisecond),
		Multiplier:   1.0,
	})

	stream, err := rm.Stream(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if stream == nil {
		t.Fatal("expected non-nil stream")
	}
}

func TestResilientModel_WithToolsPreservesWrapper(t *testing.T) {
	inner := &mockModel{}
	cb := NewCircuitBreaker(CircuitBreakerConfig{})
	rm := NewResilientModel(inner, cb, "test", nil)

	wrapped, err := rm.WithTools([]*schema.ToolInfo{{Name: "test"}})
	if err != nil {
		t.Fatalf("WithTools: %v", err)
	}

	_, ok := wrapped.(*ResilientModel)
	if !ok {
		t.Fatalf("expected *ResilientModel, got %T", wrapped)
	}
}

func TestResilientModel_BackoffTiming(t *testing.T) {
	var calls atomic.Int32
	inner := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			if calls.Add(1) <= 2 {
				return nil, fmt.Errorf("connection timeout")
			}
			return &schema.Message{Content: "ok"}, nil
		},
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 10})
	rm := NewResilientModel(inner, cb, "test", &config.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: config.Duration(50 * time.Millisecond),
		MaxDelay:     config.Duration(500 * time.Millisecond),
		Multiplier:   2.0,
	})

	start := time.Now()
	_, err := rm.Generate(context.Background(), nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	// With 2 retries and backoff ~50ms + ~100ms (±25% jitter), expect at least 80ms
	if elapsed < 80*time.Millisecond {
		t.Fatalf("expected at least 80ms for backoff, got %v", elapsed)
	}
}

func TestResilientModel_ContextCancellation(t *testing.T) {
	inner := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			return nil, fmt.Errorf("connection timeout")
		},
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 10})
	rm := NewResilientModel(inner, cb, "test", &config.RetryConfig{
		MaxAttempts:  5,
		InitialDelay: config.Duration(time.Second), // long delay
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := rm.Generate(ctx, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- FallbackModel tests ---

func TestFallbackModel_UsePrimaryOnSuccess(t *testing.T) {
	primary := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			return &schema.Message{Content: "primary"}, nil
		},
	}
	fallback := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			return &schema.Message{Content: "fallback"}, nil
		},
	}

	fm := NewFallbackModel(primary, fallback, "test")
	msg, err := fm.Generate(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Content != "primary" {
		t.Fatalf("expected 'primary', got %q", msg.Content)
	}
}

func TestFallbackModel_FallbackOnCircuitOpen(t *testing.T) {
	primary := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			return nil, fmt.Errorf("%w: provider test", ErrCircuitOpen)
		},
	}
	fallback := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			return &schema.Message{Content: "fallback"}, nil
		},
	}

	fm := NewFallbackModel(primary, fallback, "test")
	msg, err := fm.Generate(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected fallback success, got: %v", err)
	}
	if msg.Content != "fallback" {
		t.Fatalf("expected 'fallback', got %q", msg.Content)
	}
}

func TestFallbackModel_NoFallbackOnNonCircuitError(t *testing.T) {
	primary := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			return nil, fmt.Errorf("401 unauthorized")
		},
	}
	var fallbackCalled atomic.Int32
	fallback := &mockModel{
		generateFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
			fallbackCalled.Add(1)
			return &schema.Message{Content: "fallback"}, nil
		},
	}

	fm := NewFallbackModel(primary, fallback, "test")
	_, err := fm.Generate(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error (no fallback on auth error)")
	}
	if fallbackCalled.Load() != 0 {
		t.Fatal("fallback should not be called for non-circuit errors")
	}
}

func TestFallbackModel_StreamFallbackOnCircuitOpen(t *testing.T) {
	primary := &mockModel{
		streamFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
			return nil, fmt.Errorf("%w: provider test", ErrCircuitOpen)
		},
	}
	fallback := &mockModel{
		streamFunc: func(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
			sr, sw := schema.Pipe[*schema.Message](1)
			go func() {
				sw.Send(&schema.Message{Content: "fallback stream"}, nil)
				sw.Close()
			}()
			return sr, nil
		},
	}

	fm := NewFallbackModel(primary, fallback, "test")
	stream, err := fm.Stream(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected fallback stream, got: %v", err)
	}
	if stream == nil {
		t.Fatal("expected non-nil stream")
	}
}

func TestFallbackModel_WithToolsPreservesBoth(t *testing.T) {
	primary := &mockModel{}
	fallback := &mockModel{}
	fm := NewFallbackModel(primary, fallback, "test")

	wrapped, err := fm.WithTools([]*schema.ToolInfo{{Name: "tool1"}})
	if err != nil {
		t.Fatalf("WithTools: %v", err)
	}

	_, ok := wrapped.(*FallbackModel)
	if !ok {
		t.Fatalf("expected *FallbackModel, got %T", wrapped)
	}
}

// --- Registry resilience integration tests ---

func TestRegistry_FallbackProvider(t *testing.T) {
	// This tests the wiring: primary with fallback config should return a FallbackModel.
	// We can't easily test with real models (need API keys), so we test the structure.
	cfg := config.ModelsConfig{
		Default: "primary",
		Providers: map[string]config.ProviderConfig{
			"primary": {
				Driver:   "openai",
				Model:    "gpt-4o",
				Fallback: "secondary",
				Retry:    &config.RetryConfig{MaxAttempts: 2},
			},
			"secondary": {
				Driver: "openai",
				Model:  "gpt-3.5-turbo",
			},
		},
	}

	reg := NewRegistry(cfg, nil)

	// Verify entries are registered
	reg.mu.RLock()
	_, hasPrimary := reg.providers["primary"]
	_, hasSecondary := reg.providers["secondary"]
	reg.mu.RUnlock()

	if !hasPrimary || !hasSecondary {
		t.Fatal("expected both providers registered")
	}
}

func TestRegistry_NoFallbackRecursion(t *testing.T) {
	// Even if fallback also has a fallback, Get should not chain infinitely.
	cfg := config.ModelsConfig{
		Default: "a",
		Providers: map[string]config.ProviderConfig{
			"a": {
				Driver:   "openai",
				Model:    "gpt-4o",
				Fallback: "b",
				Retry:    &config.RetryConfig{MaxAttempts: 1},
			},
			"b": {
				Driver:   "openai",
				Model:    "gpt-3.5-turbo",
				Fallback: "a", // circular — should NOT cause deadlock
				Retry:    &config.RetryConfig{MaxAttempts: 1},
			},
		},
	}

	reg := NewRegistry(cfg, nil)
	_ = reg // just verify it doesn't panic during construction
}
