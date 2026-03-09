package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/config"
)

// ResilientModel wraps a ToolCallingChatModel with retry and circuit breaker.
// Retry uses exponential backoff with jitter. The circuit breaker tracks
// consecutive retryable failures and opens to fail fast when a provider is down.
type ResilientModel struct {
	inner        model.ToolCallingChatModel
	cb           *CircuitBreaker
	name         string
	maxAttempts  int
	initialDelay time.Duration
	maxDelay     time.Duration
	multiplier   float64
}

// NewResilientModel creates a resilient wrapper around the given model.
func NewResilientModel(inner model.ToolCallingChatModel, cb *CircuitBreaker, name string, cfg *config.RetryConfig) *ResilientModel {
	m := &ResilientModel{
		inner:        inner,
		cb:           cb,
		name:         name,
		maxAttempts:  3,
		initialDelay: time.Second,
		maxDelay:     30 * time.Second,
		multiplier:   2.0,
	}
	if cfg != nil {
		if cfg.MaxAttempts > 0 {
			m.maxAttempts = cfg.MaxAttempts
		}
		if cfg.InitialDelay.Duration() > 0 {
			m.initialDelay = cfg.InitialDelay.Duration()
		}
		if cfg.MaxDelay.Duration() > 0 {
			m.maxDelay = cfg.MaxDelay.Duration()
		}
		if cfg.Multiplier > 0 {
			m.multiplier = cfg.Multiplier
		}
	}
	return m
}

func (m *ResilientModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if !m.cb.Allow() {
		return nil, fmt.Errorf("%w: provider %s", ErrCircuitOpen, m.name)
	}

	var lastErr error
	for attempt := 0; attempt < m.maxAttempts; attempt++ {
		if attempt > 0 {
			delay := m.backoff(attempt, lastErr)
			slog.Debug("retrying model call", "provider", m.name, "attempt", attempt+1, "delay", delay)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			// Re-check circuit breaker before retry
			if !m.cb.Allow() {
				return nil, fmt.Errorf("%w: provider %s", ErrCircuitOpen, m.name)
			}
		}

		msg, err := m.inner.Generate(ctx, input, opts...)
		if err == nil {
			m.cb.RecordSuccess()
			return msg, nil
		}

		if !IsRetryable(err) {
			return nil, err
		}

		lastErr = err
		m.cb.RecordFailure()
	}

	return nil, fmt.Errorf("provider %s: retries exhausted after %d attempts: %w", m.name, m.maxAttempts, lastErr)
}

func (m *ResilientModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if !m.cb.Allow() {
		return nil, fmt.Errorf("%w: provider %s", ErrCircuitOpen, m.name)
	}

	var lastErr error
	for attempt := 0; attempt < m.maxAttempts; attempt++ {
		if attempt > 0 {
			delay := m.backoff(attempt, lastErr)
			slog.Debug("retrying model stream", "provider", m.name, "attempt", attempt+1, "delay", delay)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			if !m.cb.Allow() {
				return nil, fmt.Errorf("%w: provider %s", ErrCircuitOpen, m.name)
			}
		}

		stream, err := m.inner.Stream(ctx, input, opts...)
		if err == nil {
			// Stream started successfully — no retry after this point.
			m.cb.RecordSuccess()
			return stream, nil
		}

		if !IsRetryable(err) {
			return nil, err
		}

		lastErr = err
		m.cb.RecordFailure()
	}

	return nil, fmt.Errorf("provider %s: retries exhausted after %d attempts: %w", m.name, m.maxAttempts, lastErr)
}

func (m *ResilientModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	inner, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &ResilientModel{
		inner:        inner,
		cb:           m.cb,
		name:         m.name,
		maxAttempts:  m.maxAttempts,
		initialDelay: m.initialDelay,
		maxDelay:     m.maxDelay,
		multiplier:   m.multiplier,
	}, nil
}

// backoff calculates the delay before the next retry.
// Formula: min(initialDelay * multiplier^attempt, maxDelay) ± 25% jitter.
// Rate-limited errors get a minimum 5s backoff.
func (m *ResilientModel) backoff(attempt int, lastErr error) time.Duration {
	delay := time.Duration(float64(m.initialDelay) * math.Pow(m.multiplier, float64(attempt-1)))
	if delay > m.maxDelay {
		delay = m.maxDelay
	}

	// Rate limit: minimum 5s backoff
	if IsRateLimit(lastErr) && delay < 5*time.Second {
		delay = 5 * time.Second
	}

	// Jitter ±25%
	quarter := int64(delay) / 4
	if quarter > 0 {
		jitter := rand.Int64N(2*quarter) - quarter
		delay += time.Duration(jitter)
	}

	return delay
}

var _ model.ToolCallingChatModel = (*ResilientModel)(nil)

// FallbackModel tries the primary model and falls back to a secondary
// when the primary's circuit breaker is open.
type FallbackModel struct {
	primary      model.ToolCallingChatModel
	fallback     model.ToolCallingChatModel
	primaryName  string
}

// NewFallbackModel creates a model that falls back when primary's circuit is open.
func NewFallbackModel(primary, fallback model.ToolCallingChatModel, primaryName string) *FallbackModel {
	return &FallbackModel{primary: primary, fallback: fallback, primaryName: primaryName}
}

func (m *FallbackModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	msg, err := m.primary.Generate(ctx, input, opts...)
	if err != nil && errors.Is(err, ErrCircuitOpen) {
		slog.Warn("primary circuit open, falling back", "provider", m.primaryName)
		return m.fallback.Generate(ctx, input, opts...)
	}
	return msg, err
}

func (m *FallbackModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	stream, err := m.primary.Stream(ctx, input, opts...)
	if err != nil && errors.Is(err, ErrCircuitOpen) {
		slog.Warn("primary circuit open, falling back (stream)", "provider", m.primaryName)
		return m.fallback.Stream(ctx, input, opts...)
	}
	return stream, err
}

func (m *FallbackModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	primary, err := m.primary.WithTools(tools)
	if err != nil {
		return nil, err
	}
	fallback, err := m.fallback.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &FallbackModel{primary: primary, fallback: fallback, primaryName: m.primaryName}, nil
}

var _ model.ToolCallingChatModel = (*FallbackModel)(nil)
