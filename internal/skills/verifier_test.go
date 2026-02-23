package skills

import (
	"testing"
)

func TestBuildVerifyPrompt(t *testing.T) {
	criteria := &AcceptanceCriteria{
		Criteria: []string{"Contains greeting", "Is polite"},
	}

	prompt := buildVerifyPrompt(criteria, "Greet user", "Hello! How are you?")

	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}

	// Check that criteria are included
	if !contains(prompt, "Contains greeting") {
		t.Fatal("expected prompt to contain criterion 'Contains greeting'")
	}
	if !contains(prompt, "Is polite") {
		t.Fatal("expected prompt to contain criterion 'Is polite'")
	}
	if !contains(prompt, "Hello! How are you?") {
		t.Fatal("expected prompt to contain the output")
	}
}

func TestBuildVerifyPrompt_LongOutput(t *testing.T) {
	criteria := &AcceptanceCriteria{
		Criteria: []string{"test"},
	}

	// Create output longer than 4000 chars
	longOutput := ""
	for i := 0; i < 500; i++ {
		longOutput += "0123456789"
	}

	prompt := buildVerifyPrompt(criteria, "Test", longOutput)
	if !contains(prompt, "truncated") {
		t.Fatal("expected long output to be truncated")
	}
}

func TestParseVerifyResponse_ValidJSON(t *testing.T) {
	response := `{"pass": true, "score": 85, "issues": [], "feedback": "Looks good"}`
	vr := parseVerifyResponse(response)

	if !vr.Pass {
		t.Fatal("expected pass=true")
	}
	if vr.Score != 85 {
		t.Fatalf("expected score 85, got %d", vr.Score)
	}
	if vr.Feedback != "Looks good" {
		t.Fatalf("expected feedback %q, got %q", "Looks good", vr.Feedback)
	}
}

func TestParseVerifyResponse_WithCodeFences(t *testing.T) {
	response := "```json\n{\"pass\": false, \"score\": 30, \"issues\": [\"Missing header\"], \"feedback\": \"Need improvement\"}\n```"
	vr := parseVerifyResponse(response)

	if vr.Pass {
		t.Fatal("expected pass=false")
	}
	if vr.Score != 30 {
		t.Fatalf("expected score 30, got %d", vr.Score)
	}
	if len(vr.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(vr.Issues))
	}
}

func TestParseVerifyResponse_InvalidJSON(t *testing.T) {
	response := "This is not JSON at all"
	vr := parseVerifyResponse(response)

	// Should default to pass (fallback)
	if !vr.Pass {
		t.Fatal("expected fallback to pass=true for invalid JSON")
	}
	if vr.Score != 50 {
		t.Fatalf("expected fallback score 50, got %d", vr.Score)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) && searchContains(s, substr)
}

func searchContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
