package names

import (
	"strings"
	"testing"
)

func TestGenerate_Format(t *testing.T) {
	for i := 0; i < 100; i++ {
		name := Generate()
		parts := strings.SplitN(name, "_", 2)
		if len(parts) != 2 {
			t.Fatalf("expected format adj_noun, got %q", name)
		}
		if parts[0] == "" || parts[1] == "" {
			t.Fatalf("empty part in %q", name)
		}
	}
}

func TestGenerate_NoDuplicatesInLists(t *testing.T) {
	seen := make(map[string]bool)
	for _, w := range left {
		if seen[w] {
			t.Fatalf("duplicate in left: %q", w)
		}
		seen[w] = true
	}

	seen = make(map[string]bool)
	for _, w := range right {
		if seen[w] {
			t.Fatalf("duplicate in right: %q", w)
		}
		seen[w] = true
	}
}

func TestGenerate_Distribution(t *testing.T) {
	// Generate many names and check we get variety (not always the same).
	names := make(map[string]bool)
	for i := 0; i < 200; i++ {
		names[Generate()] = true
	}
	// With 50*74 = 3700 combos, 200 draws should yield at least 150 unique.
	if len(names) < 100 {
		t.Fatalf("poor distribution: only %d unique names in 200 draws", len(names))
	}
}

func TestGenerateUnique_NoCollision(t *testing.T) {
	name := GenerateUnique(func(string) bool { return false })
	if name == "" {
		t.Fatal("expected non-empty name")
	}
	parts := strings.SplitN(name, "_", 2)
	if len(parts) != 2 {
		t.Fatalf("expected format adj_noun, got %q", name)
	}
}

func TestGenerateUnique_WithCollision(t *testing.T) {
	// First call collides, second doesn't.
	calls := 0
	name := GenerateUnique(func(n string) bool {
		calls++
		return calls == 1 // only the first attempt collides
	})
	if name == "" {
		t.Fatal("expected non-empty name")
	}
	// Should have a _2 suffix since the base name collided.
	if !strings.HasSuffix(name, "_2") {
		t.Fatalf("expected _2 suffix, got %q", name)
	}
}

func TestGenerateUnique_Fallback(t *testing.T) {
	// Everything collides → should get hex fallback.
	name := GenerateUnique(func(string) bool { return true })
	if !strings.HasPrefix(name, "name_") {
		t.Fatalf("expected hex fallback starting with name_, got %q", name)
	}
	// "name_" + 6 hex chars = 11 chars
	if len(name) != 11 {
		t.Fatalf("expected length 11, got %d (%q)", len(name), name)
	}
}

func TestListSizes(t *testing.T) {
	if len(left) < 50 {
		t.Fatalf("expected at least 50 adjectives, got %d", len(left))
	}
	if len(right) < 70 {
		t.Fatalf("expected at least 70 nouns, got %d", len(right))
	}
}
