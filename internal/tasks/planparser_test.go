package tasks

import "testing"

func TestParsePlanFromMarkdown_NumberedList(t *testing.T) {
	md := `Here is the implementation plan:

1. Create the config module with default settings
2. Add the HTTP server with WebSocket support
3. Wire everything together in main.go
`
	plan := ParsePlanFromMarkdown(md)
	if plan == nil {
		t.Fatal("expected a plan, got nil")
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Title != "Create the config module with default settings" {
		t.Errorf("step 1 title = %q", plan.Steps[0].Title)
	}
	if plan.Steps[0].ID != "step_1" {
		t.Errorf("step 1 ID = %q", plan.Steps[0].ID)
	}
	if plan.Steps[0].Status != TaskPending {
		t.Errorf("step 1 status = %q", plan.Steps[0].Status)
	}
}

func TestParsePlanFromMarkdown_HeaderSteps(t *testing.T) {
	md := `# Implementation Plan

### Step 1: Setup project structure
Create directories and initialize go.mod.

### Step 2: Implement core logic
Write the main processing pipeline.

### Step 3: Add tests
Cover critical paths with unit tests.
`
	plan := ParsePlanFromMarkdown(md)
	if plan == nil {
		t.Fatal("expected a plan, got nil")
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Title != "Setup project structure" {
		t.Errorf("step 1 title = %q", plan.Steps[0].Title)
	}
	if plan.Steps[1].Description == "" {
		t.Error("step 2 expected non-empty description")
	}
}

func TestParsePlanFromMarkdown_TooFewSteps(t *testing.T) {
	md := `Just one thing:

1. Do everything at once
`
	plan := ParsePlanFromMarkdown(md)
	if plan != nil {
		t.Errorf("expected nil for single-step plan, got %d steps", len(plan.Steps))
	}
}

func TestParsePlanFromMarkdown_NoStructure(t *testing.T) {
	md := `This is just a free-text description with no numbered steps or headers.
It talks about what we should do but doesn't have a structured plan.`

	plan := ParsePlanFromMarkdown(md)
	if plan != nil {
		t.Errorf("expected nil for unstructured text, got %d steps", len(plan.Steps))
	}
}

func TestParsePlanFromMarkdown_ParenthesisNumbers(t *testing.T) {
	md := `Plan:
1) First step
2) Second step
3) Third step`

	plan := ParsePlanFromMarkdown(md)
	if plan == nil {
		t.Fatal("expected a plan, got nil")
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
}

func TestParsePlanFromMarkdown_StepDescriptions(t *testing.T) {
	md := `1. Create the module
   Set up the basic structure with go.mod

2. Add handlers
   Implement HTTP endpoints

3. Write tests
   Cover the main flow`

	plan := ParsePlanFromMarkdown(md)
	if plan == nil {
		t.Fatal("expected a plan, got nil")
	}
	// Descriptions should contain text after the title line
	if plan.Steps[0].Description == "" {
		t.Error("step 1 expected non-empty description")
	}
}

func TestParsePlanFromMarkdown_PreferHeadersOverNumbered(t *testing.T) {
	md := `### Step 1: Setup
Details here

### Step 2: Implement
More details

Some numbered stuff:
1. Sub-item A
2. Sub-item B`

	plan := ParsePlanFromMarkdown(md)
	if plan == nil {
		t.Fatal("expected a plan, got nil")
	}
	// Should prefer headers (2 steps) over numbered items (which would give more)
	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 header steps, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Title != "Setup" {
		t.Errorf("step 1 title = %q, want Setup", plan.Steps[0].Title)
	}
}
