package skills

import "fmt"

// WorkflowDef describes a structured DAG workflow loaded from workflow.yaml.
type WorkflowDef struct {
	Model string           `yaml:"model,omitempty"`
	Vars  map[string]VarDef `yaml:"vars,omitempty"`
	Steps []StepDef        `yaml:"steps"`
}

// VarDef describes a workflow input variable.
type VarDef struct {
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default,omitempty"`
}

// StepDef describes a single step in a workflow.
type StepDef struct {
	ID          string         `yaml:"id"`
	Title       string         `yaml:"title,omitempty"`
	Instruction string         `yaml:"instruction"`
	Tools       []string       `yaml:"tools,omitempty"`
	Model       string         `yaml:"model,omitempty"`
	Needs       []string       `yaml:"needs,omitempty"`
	Acceptance  *AcceptanceDef `yaml:"acceptance,omitempty"`
}

// AcceptanceDef describes acceptance criteria for a workflow step.
type AcceptanceDef struct {
	Criteria    []string `yaml:"criteria"`
	MaxAttempts int      `yaml:"max_attempts,omitempty"`
	Model       string   `yaml:"model,omitempty"`
}

// ToAcceptanceCriteria converts to the existing AcceptanceCriteria type used by the DAG engine.
func (a *AcceptanceDef) ToAcceptanceCriteria() *AcceptanceCriteria {
	if a == nil || len(a.Criteria) == 0 {
		return nil
	}
	return &AcceptanceCriteria{
		Criteria:    a.Criteria,
		MaxAttempts: a.MaxAttempts,
		Model:       a.Model,
	}
}

// ToVar converts a VarDef to the existing Var type.
func (v VarDef) ToVar() Var {
	return Var(v)
}

// ToStep converts a StepDef to the existing Step type used by the DAG engine.
func (s StepDef) ToStep() Step {
	return Step{
		ID:          s.ID,
		Title:       s.Title,
		Instruction: s.Instruction,
		Tools:       s.Tools,
		Model:       s.Model,
		Needs:       s.Needs,
		Acceptance:  s.Acceptance.ToAcceptanceCriteria(),
	}
}

// validateWorkflowDef checks a workflow definition for consistency.
func validateWorkflowDef(skillName string, w *WorkflowDef) error {
	if len(w.Steps) == 0 {
		return fmt.Errorf("skill %q: workflow requires at least one step", skillName)
	}

	ids := make(map[string]bool, len(w.Steps))
	for _, step := range w.Steps {
		if step.ID == "" {
			return fmt.Errorf("skill %q: step ID is required", skillName)
		}
		if ids[step.ID] {
			return fmt.Errorf("skill %q: duplicate step ID %q", skillName, step.ID)
		}
		ids[step.ID] = true
	}

	for _, step := range w.Steps {
		for _, need := range step.Needs {
			if !ids[need] {
				return fmt.Errorf("skill %q: step %q depends on unknown step %q", skillName, step.ID, need)
			}
			if need == step.ID {
				return fmt.Errorf("skill %q: step %q cannot depend on itself", skillName, step.ID)
			}
		}
	}

	for _, step := range w.Steps {
		if step.Instruction == "" {
			return fmt.Errorf("skill %q: step %q requires an instruction", skillName, step.ID)
		}
	}

	return nil
}
