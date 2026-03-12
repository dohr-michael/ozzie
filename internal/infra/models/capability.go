package models

import "fmt"

// Capability represents a declared model capability.
type Capability string

const (
	CapThinking    Capability = "thinking"    // extended thinking / chain-of-thought
	CapVision      Capability = "vision"      // image/multimodal input
	CapToolUse     Capability = "tool_use"    // function/tool calling
	CapCoding      Capability = "coding"      // code generation optimized
	CapLongContext Capability = "long_context" // >100K token context
	CapFast        Capability = "fast"         // low-latency inference
	CapCheap       Capability = "cheap"        // cost-optimized
	CapWriting     Capability = "writing"      // text/content generation
)

// AllCapabilities lists all known capabilities (for validation).
var AllCapabilities = []Capability{
	CapThinking, CapVision, CapToolUse, CapCoding,
	CapLongContext, CapFast, CapCheap, CapWriting,
}

// IsValid returns true if the capability is a known value.
func (c Capability) IsValid() bool {
	for _, known := range AllCapabilities {
		if c == known {
			return true
		}
	}
	return false
}

// ValidateCapabilities logs a warning for unknown capabilities.
// Returns an error only if a capability string is empty.
func ValidateCapabilities(caps []string) error {
	for _, c := range caps {
		if c == "" {
			return fmt.Errorf("empty capability string")
		}
		if !Capability(c).IsValid() {
			return fmt.Errorf("unknown capability %q", c)
		}
	}
	return nil
}
