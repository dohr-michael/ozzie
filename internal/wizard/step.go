package wizard

import tea "github.com/charmbracelet/bubbletea"

// StepDoneMsg signals the wizard to advance to the next step.
type StepDoneMsg struct{}

// StepBackMsg signals the wizard to go back to the previous step.
type StepBackMsg struct{}

// Step represents a single wizard step.
type Step interface {
	// ID returns a unique identifier (e.g. "provider", "apikey").
	ID() string

	// Title returns the step title for the progress bar.
	Title() string

	// Init initializes the step with collected answers so far.
	// Used for pre-filling defaults and conditional logic.
	Init(answers Answers) tea.Cmd

	// Update handles a tea.Msg.
	// Returns StepDoneMsg when complete, StepBackMsg to go back.
	Update(msg tea.Msg) (Step, tea.Cmd)

	// View renders the step content.
	View() string

	// Collect returns the answers collected by this step.
	Collect() Answers

	// ShouldSkip returns true if this step should be skipped.
	ShouldSkip(answers Answers) bool
}

// Answers is a typed map of collected wizard answers.
type Answers map[string]any

// String returns a string value from answers with a default fallback.
func (a Answers) String(key, fallback string) string {
	if v, ok := a[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

// Int returns an int value from answers with a default fallback.
func (a Answers) Int(key string, fallback int) int {
	if v, ok := a[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case float64:
			return int(n)
		}
	}
	return fallback
}

// Bool returns a bool value from answers with a default fallback.
func (a Answers) Bool(key string, fallback bool) bool {
	if v, ok := a[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return fallback
}

// Strings returns a []string value from answers with a default fallback.
func (a Answers) Strings(key string, fallback []string) []string {
	if v, ok := a[key]; ok {
		if s, ok := v.([]string); ok {
			return s
		}
	}
	return fallback
}

// Merge copies all entries from other into a.
func (a Answers) Merge(other Answers) {
	for k, v := range other {
		a[k] = v
	}
}
