package skills

import (
	"fmt"
	"os"
	"strings"

	"github.com/marcozac/go-jsonc"
)

// SkillType distinguishes simple (single agent) from workflow (DAG) skills.
type SkillType string

const (
	SkillTypeSimple   SkillType = "simple"
	SkillTypeWorkflow SkillType = "workflow"
)

// Skill represents a declarative skill loaded from JSONC.
type Skill struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        SkillType         `json:"type"`
	Version     int               `json:"version"`
	Model       string            `json:"model"`       // for simple + default for workflow steps
	Instruction string            `json:"instruction"` // for simple skills
	Tools       []string          `json:"tools"`       // tool names available to this skill
	Triggers    TriggerConfig     `json:"triggers"`
	Vars        map[string]Var    `json:"vars"`
	Steps       []Step            `json:"steps"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TriggerConfig controls how a skill can be invoked.
type TriggerConfig struct {
	Delegation bool `json:"delegation"`
}

// Var describes a skill input variable.
type Var struct {
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default"`
}

// Step describes a single step in a workflow skill.
type Step struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Instruction string   `json:"instruction"`
	Tools       []string `json:"tools"`
	Model       string   `json:"model"`
	Needs       []string `json:"needs"`
	Acceptance  string   `json:"acceptance"`
}

// LoadSkill reads a JSONC skill definition from disk.
func LoadSkill(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill %s: %w", path, err)
	}

	var s Skill
	if err := jsonc.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse skill %s: %w", path, err)
	}

	// Infer type from steps
	if s.Type == "" {
		if len(s.Steps) > 0 {
			s.Type = SkillTypeWorkflow
		} else {
			s.Type = SkillTypeSimple
		}
	}

	// Default triggers
	if !s.Triggers.Delegation {
		s.Triggers.Delegation = true
	}

	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("validate skill %s: %w", path, err)
	}

	return &s, nil
}

// Validate checks the skill definition for consistency.
func (s *Skill) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if s.Description == "" {
		return fmt.Errorf("skill %q: description is required", s.Name)
	}

	switch s.Type {
	case SkillTypeSimple:
		if s.Instruction == "" {
			return fmt.Errorf("skill %q: simple skill requires an instruction", s.Name)
		}
	case SkillTypeWorkflow:
		if len(s.Steps) == 0 {
			return fmt.Errorf("skill %q: workflow skill requires at least one step", s.Name)
		}
		if err := s.validateSteps(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("skill %q: unknown type %q", s.Name, s.Type)
	}

	return nil
}

func (s *Skill) validateSteps() error {
	ids := make(map[string]bool, len(s.Steps))

	// Collect all step IDs
	for _, step := range s.Steps {
		if step.ID == "" {
			return fmt.Errorf("skill %q: step ID is required", s.Name)
		}
		if ids[step.ID] {
			return fmt.Errorf("skill %q: duplicate step ID %q", s.Name, step.ID)
		}
		ids[step.ID] = true
	}

	// Validate needs references
	for _, step := range s.Steps {
		for _, need := range step.Needs {
			if !ids[need] {
				return fmt.Errorf("skill %q: step %q depends on unknown step %q", s.Name, step.ID, need)
			}
			if need == step.ID {
				return fmt.Errorf("skill %q: step %q cannot depend on itself", s.Name, step.ID)
			}
		}
	}

	// Validate step instructions
	for _, step := range s.Steps {
		if step.Instruction == "" {
			return fmt.Errorf("skill %q: step %q requires an instruction", s.Name, step.ID)
		}
	}

	return nil
}

// String returns a human-readable representation of the skill.
func (s *Skill) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s (%s)", s.Name, s.Type))
	if len(s.Steps) > 0 {
		sb.WriteString(fmt.Sprintf(", %d steps", len(s.Steps)))
	}
	return sb.String()
}
