package models

import (
	"sync"
	"time"
)

// cbState represents the circuit breaker state.
type cbState int

const (
	cbClosed   cbState = iota // normal operation
	cbOpen                    // rejecting requests
	cbHalfOpen                // testing with a single request
)

// CircuitBreakerConfig configures a circuit breaker.
type CircuitBreakerConfig struct {
	Threshold int           // consecutive failures before opening (default: 5)
	Cooldown  time.Duration // time in open state before half-open (default: 30s)
}

func (c CircuitBreakerConfig) withDefaults() CircuitBreakerConfig {
	if c.Threshold <= 0 {
		c.Threshold = 5
	}
	if c.Cooldown <= 0 {
		c.Cooldown = 30 * time.Second
	}
	return c
}

// CircuitBreaker implements the circuit breaker pattern.
//
// States:
//   - Closed: normal operation, counts consecutive failures.
//   - Open: rejects immediately; transitions to half-open after cooldown.
//   - Half-open: allows one probe request; success → closed, failure → open.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            cbState
	consecutiveFails int
	lastFailure      time.Time
	cfg              CircuitBreakerConfig
}

// NewCircuitBreaker creates a circuit breaker with the given config.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{cfg: cfg.withDefaults()}
}

// Allow returns true if the request should be allowed through.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case cbClosed:
		return true
	case cbOpen:
		if time.Since(cb.lastFailure) >= cb.cfg.Cooldown {
			cb.state = cbHalfOpen
			return true
		}
		return false
	case cbHalfOpen:
		// Only one request allowed in half-open; block further requests
		// until the probe completes (RecordSuccess/RecordFailure).
		return false
	}
	return false
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFails = 0
	cb.state = cbClosed
}

// RecordFailure records a failed request (retryable errors only).
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFails++
	cb.lastFailure = time.Now()
	if cb.consecutiveFails >= cb.cfg.Threshold || cb.state == cbHalfOpen {
		cb.state = cbOpen
	}
}

// State returns the current circuit breaker state (for testing/monitoring).
func (cb *CircuitBreaker) State() cbState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
