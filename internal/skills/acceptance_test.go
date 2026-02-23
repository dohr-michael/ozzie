package skills

import (
	"encoding/json"
	"testing"
)

func TestAcceptanceCriteria_UnmarshalString(t *testing.T) {
	data := []byte(`"Output contains greeting"`)
	var ac AcceptanceCriteria
	if err := json.Unmarshal(data, &ac); err != nil {
		t.Fatalf("UnmarshalJSON string: %v", err)
	}
	if len(ac.Criteria) != 1 {
		t.Fatalf("expected 1 criterion, got %d", len(ac.Criteria))
	}
	if ac.Criteria[0] != "Output contains greeting" {
		t.Fatalf("expected criterion %q, got %q", "Output contains greeting", ac.Criteria[0])
	}
	if ac.MaxAttempts != 0 {
		t.Fatalf("expected max_attempts 0 (default), got %d", ac.MaxAttempts)
	}
}

func TestAcceptanceCriteria_UnmarshalObject(t *testing.T) {
	data := []byte(`{
		"criteria": ["Has title", "Has body"],
		"max_attempts": 3,
		"model": "haiku"
	}`)
	var ac AcceptanceCriteria
	if err := json.Unmarshal(data, &ac); err != nil {
		t.Fatalf("UnmarshalJSON object: %v", err)
	}
	if len(ac.Criteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d", len(ac.Criteria))
	}
	if ac.MaxAttempts != 3 {
		t.Fatalf("expected max_attempts 3, got %d", ac.MaxAttempts)
	}
	if ac.Model != "haiku" {
		t.Fatalf("expected model %q, got %q", "haiku", ac.Model)
	}
}

func TestAcceptanceCriteria_UnmarshalEmptyString(t *testing.T) {
	data := []byte(`""`)
	var ac AcceptanceCriteria
	if err := json.Unmarshal(data, &ac); err != nil {
		t.Fatalf("UnmarshalJSON empty string: %v", err)
	}
	if ac.HasCriteria() {
		t.Fatal("expected HasCriteria() false for empty string")
	}
}

func TestAcceptanceCriteria_HasCriteria(t *testing.T) {
	var nilAC *AcceptanceCriteria
	if nilAC.HasCriteria() {
		t.Fatal("expected nil HasCriteria to be false")
	}

	empty := &AcceptanceCriteria{}
	if empty.HasCriteria() {
		t.Fatal("expected empty HasCriteria to be false")
	}

	withCriteria := &AcceptanceCriteria{Criteria: []string{"test"}}
	if !withCriteria.HasCriteria() {
		t.Fatal("expected HasCriteria to be true with criteria")
	}
}

func TestAcceptanceCriteria_EffectiveMaxAttempts(t *testing.T) {
	var nilAC *AcceptanceCriteria
	if nilAC.EffectiveMaxAttempts() != 2 {
		t.Fatalf("expected default 2, got %d", nilAC.EffectiveMaxAttempts())
	}

	zero := &AcceptanceCriteria{}
	if zero.EffectiveMaxAttempts() != 2 {
		t.Fatalf("expected default 2 for zero, got %d", zero.EffectiveMaxAttempts())
	}

	custom := &AcceptanceCriteria{MaxAttempts: 5}
	if custom.EffectiveMaxAttempts() != 5 {
		t.Fatalf("expected 5, got %d", custom.EffectiveMaxAttempts())
	}
}

func TestAcceptanceCriteria_InSkillStep(t *testing.T) {
	// Test that Step can unmarshal with the new AcceptanceCriteria type
	data := []byte(`{
		"id": "s1",
		"title": "Step 1",
		"instruction": "Do stuff",
		"acceptance": {"criteria": ["Output is valid"], "max_attempts": 2}
	}`)

	var step Step
	if err := json.Unmarshal(data, &step); err != nil {
		t.Fatalf("Unmarshal step: %v", err)
	}
	if !step.Acceptance.HasCriteria() {
		t.Fatal("expected step acceptance to have criteria")
	}
	if step.Acceptance.Criteria[0] != "Output is valid" {
		t.Fatalf("expected criterion %q, got %q", "Output is valid", step.Acceptance.Criteria[0])
	}
}

func TestAcceptanceCriteria_BackwardCompat(t *testing.T) {
	// Test backward compat: string acceptance in Step
	data := []byte(`{
		"id": "s1",
		"title": "Step 1",
		"instruction": "Do stuff",
		"acceptance": "Must return JSON"
	}`)

	var step Step
	if err := json.Unmarshal(data, &step); err != nil {
		t.Fatalf("Unmarshal step (string): %v", err)
	}
	if !step.Acceptance.HasCriteria() {
		t.Fatal("expected step acceptance to have criteria from string")
	}
	if step.Acceptance.Criteria[0] != "Must return JSON" {
		t.Fatalf("expected criterion %q, got %q", "Must return JSON", step.Acceptance.Criteria[0])
	}
}
