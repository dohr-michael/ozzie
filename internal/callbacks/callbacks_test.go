package callbacks

import (
	"strings"
	"testing"
)

func TestTruncatePayload_Short(t *testing.T) {
	result := truncatePayload("hello", 100)
	if result != "hello" {
		t.Fatalf("expected %q, got %q", "hello", result)
	}
}

func TestTruncatePayload_Exact(t *testing.T) {
	s := strings.Repeat("a", 50)
	result := truncatePayload(s, 50)
	if result != s {
		t.Fatalf("expected original string (len %d), got len %d", len(s), len(result))
	}
}

func TestTruncatePayload_Long(t *testing.T) {
	s := strings.Repeat("x", 200)
	result := truncatePayload(s, 100)
	if len(result) != 100+len("... (truncated)") {
		t.Fatalf("expected truncated length %d, got %d", 100+len("... (truncated)"), len(result))
	}
	if !strings.HasSuffix(result, "... (truncated)") {
		t.Fatalf("expected suffix '... (truncated)', got %q", result[len(result)-20:])
	}
	if result[:100] != strings.Repeat("x", 100) {
		t.Fatal("prefix should be first 100 chars of original")
	}
}

func TestTruncatePayload_ZeroMax(t *testing.T) {
	s := "hello world"
	result := truncatePayload(s, 0)
	if result != s {
		t.Fatalf("expected original string when maxLen=0, got %q", result)
	}
}
