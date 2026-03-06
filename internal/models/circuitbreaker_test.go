package models

import (
	"testing"
	"time"
)

func TestCircuitBreaker_DefaultsClosed(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{})
	if cb.State() != cbClosed {
		t.Fatal("expected initial state closed")
	}
	if !cb.Allow() {
		t.Fatal("expected allow in closed state")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 3, Cooldown: time.Hour})

	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("expected allow at failure %d", i)
		}
		cb.RecordFailure()
	}

	if cb.State() != cbOpen {
		t.Fatal("expected open after threshold failures")
	}
	if cb.Allow() {
		t.Fatal("expected reject in open state")
	}
}

func TestCircuitBreaker_SuccessResetsCount(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 3, Cooldown: time.Hour})

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // reset
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != cbClosed {
		t.Fatal("expected closed — success should have reset counter")
	}
}

func TestCircuitBreaker_HalfOpenAfterCooldown(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 1, Cooldown: 10 * time.Millisecond})

	cb.Allow()
	cb.RecordFailure()
	if cb.State() != cbOpen {
		t.Fatal("expected open")
	}

	time.Sleep(15 * time.Millisecond)

	if !cb.Allow() {
		t.Fatal("expected allow in half-open (probe request)")
	}
	if cb.State() != cbHalfOpen {
		t.Fatal("expected half-open state")
	}
}

func TestCircuitBreaker_HalfOpenSuccess(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 1, Cooldown: 10 * time.Millisecond})

	cb.Allow()
	cb.RecordFailure() // → open

	time.Sleep(15 * time.Millisecond)
	cb.Allow() // → half-open

	cb.RecordSuccess() // → closed
	if cb.State() != cbClosed {
		t.Fatal("expected closed after half-open success")
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 1, Cooldown: 10 * time.Millisecond})

	cb.Allow()
	cb.RecordFailure() // → open

	time.Sleep(15 * time.Millisecond)
	cb.Allow() // → half-open

	cb.RecordFailure() // → open again
	if cb.State() != cbOpen {
		t.Fatal("expected open after half-open failure")
	}
}

func TestCircuitBreaker_HalfOpenBlocksConcurrent(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 1, Cooldown: 10 * time.Millisecond})

	cb.Allow()
	cb.RecordFailure() // → open

	time.Sleep(15 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("first probe request should be allowed")
	}

	// Second request while in half-open should be blocked
	if cb.Allow() {
		t.Fatal("concurrent request in half-open should be blocked")
	}
}

func TestCircuitBreaker_DefaultConfig(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{})
	if cb.cfg.Threshold != 5 {
		t.Fatalf("expected default threshold 5, got %d", cb.cfg.Threshold)
	}
	if cb.cfg.Cooldown != 30*time.Second {
		t.Fatalf("expected default cooldown 30s, got %v", cb.cfg.Cooldown)
	}
}
