package models

import (
	"errors"
	"fmt"
	"strings"
)

// ErrCircuitOpen is returned when the circuit breaker is open for a provider.
var ErrCircuitOpen = errors.New("circuit breaker open")

// HandleError converts common SDK errors to user-friendly errors.
func HandleError(err error) error {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())

	if containsAny(errStr, "401", "403", "unauthorized", "invalid api key", "api key", "forbidden") {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if containsAny(errStr, "429", "rate limit", "quota", "too many requests") {
		return fmt.Errorf("rate limited: %w", err)
	}

	if containsAny(errStr, "context length", "too many tokens", "max tokens", "token limit") {
		return fmt.Errorf("context too long: %w", err)
	}

	if containsAny(errStr, "model not found", "404", "not found") {
		return fmt.Errorf("model not found: %w", err)
	}

	if containsAny(errStr, "connection", "eof", "timeout", "dial", "refused") {
		return fmt.Errorf("connection error: %w", err)
	}

	return err
}

// IsRetryable returns true if the error is transient and the operation should be retried.
// Non-retryable: auth errors, model not found, context too long, circuit breaker open.
// Retryable: timeout, connection, EOF, rate limit, model unavailable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, ErrCircuitOpen) {
		return false
	}

	var unavail *ErrModelUnavailable
	if errors.As(err, &unavail) {
		return true
	}

	errStr := strings.ToLower(err.Error())

	// Non-retryable errors
	if containsAny(errStr, "401", "403", "unauthorized", "invalid api key", "forbidden") {
		return false
	}
	if containsAny(errStr, "model not found", "404", "not found") {
		return false
	}
	if containsAny(errStr, "context length", "too many tokens", "token limit", "context too long") {
		return false
	}

	// Retryable errors
	if containsAny(errStr, "timeout", "deadline exceeded", "connection", "eof", "dial",
		"refused", "reset", "429", "rate limit", "too many requests", "quota",
		"502", "503", "504", "overloaded") {
		return true
	}

	return false
}

// IsRateLimit returns true if the error indicates a rate limit (429).
// Rate-limited requests use a longer minimum backoff.
func IsRateLimit(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return containsAny(errStr, "429", "rate limit", "too many requests", "quota")
}

// ErrModelUnavailable indicates the model backend returned a non-JSON or error response.
type ErrModelUnavailable struct {
	Provider string
	Body     string // raw response body (truncated)
	Cause    error  // original error if any
}

func (e *ErrModelUnavailable) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("model %s unavailable: %s", e.Provider, e.Body)
	}
	return fmt.Sprintf("model %s unavailable: %v", e.Provider, e.Cause)
}

func (e *ErrModelUnavailable) Unwrap() error { return e.Cause }

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
