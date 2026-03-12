package models

import "testing"

func TestCapabilityIsValid(t *testing.T) {
	tests := []struct {
		cap  Capability
		want bool
	}{
		{CapThinking, true},
		{CapVision, true},
		{CapToolUse, true},
		{CapCoding, true},
		{CapLongContext, true},
		{CapFast, true},
		{CapCheap, true},
		{CapWriting, true},
		{Capability("unknown"), false},
		{Capability(""), false},
	}

	for _, tt := range tests {
		got := tt.cap.IsValid()
		if got != tt.want {
			t.Errorf("Capability(%q).IsValid(): got %v, want %v", tt.cap, got, tt.want)
		}
	}
}

func TestValidateCapabilities(t *testing.T) {
	// Valid
	if err := ValidateCapabilities([]string{"coding", "tool_use"}); err != nil {
		t.Errorf("valid capabilities: unexpected error: %v", err)
	}

	// Empty slice is valid
	if err := ValidateCapabilities(nil); err != nil {
		t.Errorf("nil capabilities: unexpected error: %v", err)
	}

	// Unknown capability
	if err := ValidateCapabilities([]string{"coding", "teleportation"}); err == nil {
		t.Error("expected error for unknown capability")
	}

	// Empty string
	if err := ValidateCapabilities([]string{""}); err == nil {
		t.Error("expected error for empty capability string")
	}
}
