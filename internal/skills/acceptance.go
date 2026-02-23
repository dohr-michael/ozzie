package skills

import (
	"encoding/json"
)

// AcceptanceCriteria defines how to verify a step's output.
type AcceptanceCriteria struct {
	Criteria    []string `json:"criteria"`
	MaxAttempts int      `json:"max_attempts"`
	Model       string   `json:"model"`
}

// HasCriteria returns true if there are acceptance criteria to verify.
func (ac *AcceptanceCriteria) HasCriteria() bool {
	return ac != nil && len(ac.Criteria) > 0
}

// EffectiveMaxAttempts returns the max retry attempts, defaulting to 2.
func (ac *AcceptanceCriteria) EffectiveMaxAttempts() int {
	if ac == nil || ac.MaxAttempts <= 0 {
		return 2
	}
	return ac.MaxAttempts
}

// UnmarshalJSON supports both a simple string and a full object.
// String input is treated as a single criterion with defaults.
func (ac *AcceptanceCriteria) UnmarshalJSON(data []byte) error {
	// Try string first (backward compat)
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s != "" {
			ac.Criteria = []string{s}
		}
		return nil
	}

	// Try object
	type alias AcceptanceCriteria
	var obj alias
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*ac = AcceptanceCriteria(obj)
	return nil
}
