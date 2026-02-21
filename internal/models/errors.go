package models

import (
	"fmt"
	"strings"
)

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

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
