package skills

// Step describes a single step in a workflow DAG.
// Used internally by the DAG engine and WorkflowRunner.
type Step struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Instruction string              `json:"instruction"`
	Tools       []string            `json:"tools"`
	Model       string              `json:"model"`
	Needs       []string            `json:"needs"`
	Acceptance  *AcceptanceCriteria `json:"acceptance,omitempty"`
}

// Var describes a skill input variable.
type Var struct {
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default"`
}

// EventTrigger describes an event-based trigger for a skill.
type EventTrigger struct {
	Event  string            `yaml:"event"  json:"event"`
	Filter map[string]string `yaml:"filter,omitempty" json:"filter,omitempty"`
}
