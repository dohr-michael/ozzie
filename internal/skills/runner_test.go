package skills

import (
	"strings"
	"testing"
)

func TestValidateVars_RequiredPresent(t *testing.T) {
	wr := &WorkflowRunner{
		skill: &Skill{
			Name: "test",
			Vars: map[string]Var{
				"url": {Required: true},
			},
		},
	}

	err := wr.validateVars(map[string]string{"url": "https://example.com"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateVars_RequiredMissing(t *testing.T) {
	wr := &WorkflowRunner{
		skill: &Skill{
			Name: "test",
			Vars: map[string]Var{
				"url": {Required: true},
			},
		},
	}

	err := wr.validateVars(map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing required var")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("expected error to mention 'url', got: %v", err)
	}
}

func TestValidateVars_OptionalMissing(t *testing.T) {
	wr := &WorkflowRunner{
		skill: &Skill{
			Name: "test",
			Vars: map[string]Var{
				"format": {Required: false, Default: "markdown"},
			},
		},
	}

	err := wr.validateVars(map[string]string{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildStepInstruction_Basic(t *testing.T) {
	wr := &WorkflowRunner{skill: &Skill{Name: "test"}}
	step := &Step{
		ID:          "analyze",
		Instruction: "Analyze the code.",
	}

	result := wr.buildStepInstruction(step, nil, nil)
	if result != "Analyze the code." {
		t.Errorf("unexpected instruction: %q", result)
	}
}

func TestBuildStepInstruction_WithVars(t *testing.T) {
	wr := &WorkflowRunner{skill: &Skill{Name: "test"}}
	step := &Step{
		ID:          "analyze",
		Instruction: "Analyze the code.",
	}

	vars := map[string]string{"language": "go", "level": "strict"}
	result := wr.buildStepInstruction(step, vars, nil)

	if !strings.Contains(result, "Analyze the code.") {
		t.Error("expected base instruction")
	}
	if !strings.Contains(result, "## Variables") {
		t.Error("expected variables section")
	}
	if !strings.Contains(result, "**language**: go") {
		t.Error("expected language variable")
	}
}

func TestBuildStepInstruction_WithPrevResults(t *testing.T) {
	wr := &WorkflowRunner{skill: &Skill{Name: "test"}}
	step := &Step{
		ID:          "summarize",
		Instruction: "Summarize the analysis.",
		Needs:       []string{"analyze"},
	}

	prevResults := map[string]string{
		"analyze": "Found 3 issues in the codebase.",
	}

	result := wr.buildStepInstruction(step, nil, prevResults)

	if !strings.Contains(result, "## Previous Step Results") {
		t.Error("expected previous results section")
	}
	if !strings.Contains(result, "### Step: analyze") {
		t.Error("expected analyze step header")
	}
	if !strings.Contains(result, "Found 3 issues") {
		t.Error("expected analyze result content")
	}
}

func TestBuildStepInstruction_WithAcceptance(t *testing.T) {
	wr := &WorkflowRunner{skill: &Skill{Name: "test"}}
	step := &Step{
		ID:          "review",
		Instruction: "Review the code.",
		Acceptance:  "Output must include severity ratings.",
	}

	result := wr.buildStepInstruction(step, nil, nil)

	if !strings.Contains(result, "## Acceptance Criteria") {
		t.Error("expected acceptance criteria section")
	}
	if !strings.Contains(result, "severity ratings") {
		t.Error("expected acceptance content")
	}
}

func TestBuildStepInstruction_Full(t *testing.T) {
	wr := &WorkflowRunner{skill: &Skill{Name: "test"}}
	step := &Step{
		ID:          "final",
		Instruction: "Produce the final report.",
		Needs:       []string{"analyze", "review"},
		Acceptance:  "Must be in markdown format.",
	}

	vars := map[string]string{"project": "ozzie"}
	prevResults := map[string]string{
		"analyze": "Analysis complete.",
		"review":  "Review complete.",
	}

	result := wr.buildStepInstruction(step, vars, prevResults)

	// Check order: instruction → variables → previous results → acceptance
	instrIdx := strings.Index(result, "Produce the final report.")
	varsIdx := strings.Index(result, "## Variables")
	prevIdx := strings.Index(result, "## Previous Step Results")
	accIdx := strings.Index(result, "## Acceptance Criteria")

	if instrIdx > varsIdx || varsIdx > prevIdx || prevIdx > accIdx {
		t.Errorf("sections not in expected order: instr=%d, vars=%d, prev=%d, acc=%d",
			instrIdx, varsIdx, prevIdx, accIdx)
	}
}
