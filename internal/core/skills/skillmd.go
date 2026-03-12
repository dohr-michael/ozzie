package skills

import "fmt"

// SkillMD represents a skill loaded from the SKILL.md + optional YAML files format.
// This follows the agentskills.io specification with hybrid extensions.
type SkillMD struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license,omitempty"`
	Compatibility string            `yaml:"compatibility,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
	AllowedTools  []string          `yaml:"allowed-tools,omitempty"`
	Body          string            `yaml:"-"` // Markdown body (below frontmatter)
	Dir           string            `yaml:"-"` // Directory containing the skill files

	Workflow *WorkflowDef `yaml:"-"` // Optional: loaded from workflow.yaml
	Triggers *TriggersDef `yaml:"-"` // Optional: loaded from triggers.yaml
}

// HasWorkflow returns true if the skill has a workflow definition.
func (s *SkillMD) HasWorkflow() bool {
	return s.Workflow != nil && len(s.Workflow.Steps) > 0
}

// Validate checks the skill definition for consistency.
func (s *SkillMD) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if s.Description == "" {
		return fmt.Errorf("skill %q: description is required", s.Name)
	}
	if s.Body == "" && !s.HasWorkflow() {
		return fmt.Errorf("skill %q: body or workflow is required", s.Name)
	}
	if s.HasWorkflow() {
		if err := validateWorkflowDef(s.Name, s.Workflow); err != nil {
			return err
		}
	}
	return nil
}
