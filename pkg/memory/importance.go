package memory

import "time"

// ImportanceLevel controls how aggressively a memory decays.
type ImportanceLevel string

const (
	ImportanceCore      ImportanceLevel = "core"      // never decays
	ImportanceImportant ImportanceLevel = "important"  // slow decay
	ImportanceNormal    ImportanceLevel = "normal"     // default decay
	ImportanceEphemeral ImportanceLevel = "ephemeral"  // fast decay, auto-purge
)

// ValidImportanceLevels lists all valid importance values.
var ValidImportanceLevels = []ImportanceLevel{
	ImportanceCore, ImportanceImportant, ImportanceNormal, ImportanceEphemeral,
}

// IsValidImportance checks whether an importance level string is valid.
func IsValidImportance(s string) bool {
	for _, l := range ValidImportanceLevels {
		if string(l) == s {
			return true
		}
	}
	return false
}

// decayConfig holds per-level decay parameters.
type decayConfig struct {
	GracePeriod time.Duration
	Rate        float64 // per week after grace
	Floor       float64
}

var decayConfigs = map[ImportanceLevel]decayConfig{
	ImportanceCore:      {}, // no decay
	ImportanceImportant: {GracePeriod: 30 * 24 * time.Hour, Rate: 0.005, Floor: 0.3},
	ImportanceNormal:    {GracePeriod: 7 * 24 * time.Hour, Rate: 0.01, Floor: 0.1},
	ImportanceEphemeral: {GracePeriod: 24 * time.Hour, Rate: 0.05, Floor: 0.1},
}
