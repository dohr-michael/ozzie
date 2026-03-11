package discord

import (
	"strings"
	"testing"
)

func TestSplitMessageEmpty(t *testing.T) {
	chunks := splitMessage("", 2000)
	if len(chunks) != 1 || chunks[0] != "" {
		t.Fatalf("expected [\"\"], got %v", chunks)
	}
}

func TestSplitMessageExactly2000(t *testing.T) {
	msg := strings.Repeat("a", 2000)
	chunks := splitMessage(msg, 2000)
	if len(chunks) != 1 || chunks[0] != msg {
		t.Fatalf("expected 1 chunk of 2000 chars, got %d chunks", len(chunks))
	}
}

func TestSplitMessageShort(t *testing.T) {
	chunks := splitMessage("hello", 2000)
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Fatalf("expected [\"hello\"], got %v", chunks)
	}
}

func TestSplitMessageNewline(t *testing.T) {
	// Message slightly over limit, with newline before the limit
	part1 := strings.Repeat("a", 1990)
	part2 := strings.Repeat("b", 20)
	msg := part1 + "\n" + part2

	chunks := splitMessage(msg, 2000)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != part1 {
		t.Fatalf("chunk[0] wrong, len=%d", len(chunks[0]))
	}
	if chunks[1] != part2 {
		t.Fatalf("chunk[1] wrong: %q", chunks[1])
	}
}

func TestSplitMessageSpace(t *testing.T) {
	part1 := strings.Repeat("a", 1990)
	part2 := strings.Repeat("b", 20)
	msg := part1 + " " + part2

	chunks := splitMessage(msg, 2000)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != part1 {
		t.Fatalf("chunk[0] len=%d, want 1990", len(chunks[0]))
	}
}

func TestSplitMessageHardCut(t *testing.T) {
	// No newlines or spaces
	msg := strings.Repeat("x", 4000)
	chunks := splitMessage(msg, 2000)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 2000 {
		t.Fatalf("chunk[0] len=%d, want 2000", len(chunks[0]))
	}
	if len(chunks[1]) != 2000 {
		t.Fatalf("chunk[1] len=%d, want 2000", len(chunks[1]))
	}
}

func TestSplitMessageUnicode(t *testing.T) {
	// Unicode characters (multi-byte in UTF-8 but single chars)
	msg := strings.Repeat("é", 2001)
	chunks := splitMessage(msg, 2000)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	// Verify all content is preserved
	reassembled := strings.Join(chunks, "")
	if reassembled != msg {
		t.Fatal("content not preserved after split")
	}
}

func TestSplitMessageAllChunksUnderLimit(t *testing.T) {
	msg := strings.Repeat("word ", 1000) // 5000 chars
	chunks := splitMessage(msg, 2000)
	for i, chunk := range chunks {
		if len(chunk) > 2000 {
			t.Fatalf("chunk[%d] exceeds limit: len=%d", i, len(chunk))
		}
	}
}
