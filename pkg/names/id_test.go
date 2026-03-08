package names

import (
	"strings"
	"testing"
)

func TestGenerateID_Format(t *testing.T) {
	id := GenerateID("sess", func(string) bool { return false })
	if !strings.HasPrefix(id, "sess_") {
		t.Fatalf("expected sess_ prefix, got %q", id)
	}
	// Should have at least 3 parts: prefix + adjective + noun
	parts := strings.SplitN(id, "_", 3)
	if len(parts) < 3 {
		t.Fatalf("expected at least 3 parts (prefix_adj_noun), got %q", id)
	}
}

func TestGenerateID_Uniqueness(t *testing.T) {
	existing := map[string]bool{}
	id := GenerateID("task", func(candidate string) bool {
		return existing[candidate]
	})
	existing[id] = true

	// Second call with same exists should produce different ID
	id2 := GenerateID("task", func(candidate string) bool {
		return existing[candidate]
	})
	if id == id2 {
		t.Fatalf("expected different IDs, both are %q", id)
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"sess_cosmic_asimov", "cosmic asimov"},
		{"task_stellar_deckard_0002", "stellar deckard 0002"},
		{"mem_void_herbert", "void herbert"},
		{"noprefix", "noprefix"},
	}
	for _, tt := range tests {
		got := DisplayName(tt.id)
		if got != tt.want {
			t.Errorf("DisplayName(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}
