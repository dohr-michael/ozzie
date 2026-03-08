package names

import "strings"

// GenerateID creates a unique ID with the given prefix.
// Format: "{prefix}_{adjective}_{noun}" or "{prefix}_{adjective}_{noun}_{XXXX}"
func GenerateID(prefix string, exists func(string) bool) string {
	name := GenerateUnique(func(n string) bool {
		return exists(prefix + "_" + n)
	})
	return prefix + "_" + name
}

// DisplayName extracts the human-readable name from a prefixed ID.
// "sess_cosmic_asimov" → "cosmic asimov"
// "task_stellar_deckard_0002" → "stellar deckard 0002"
func DisplayName(id string) string {
	if i := strings.IndexByte(id, '_'); i >= 0 {
		return strings.ReplaceAll(id[i+1:], "_", " ")
	}
	return id
}
