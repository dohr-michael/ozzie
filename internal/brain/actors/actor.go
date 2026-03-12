// Package actors provides capacity-aware LLM orchestration via typed slots (actors).
package actors

// ActorStatus represents the state of an actor slot.
type ActorStatus string

const (
	ActorIdle ActorStatus = "idle"
	ActorBusy ActorStatus = "busy"
)

// Actor represents a single LLM capacity slot bound to a provider.
type Actor struct {
	ID           string      `json:"id"`
	ProviderName string      `json:"provider_name"`
	Tags         []string    `json:"tags,omitempty"`
	Capabilities []string    `json:"capabilities,omitempty"`
	PromptPrefix string      `json:"prompt_prefix,omitempty"`
	Status       ActorStatus `json:"status"`
	CurrentTask  string      `json:"current_task,omitempty"`
}

// MatchesTags returns true if the actor supports all requested tags.
// An empty request matches any actor.
func (a *Actor) MatchesTags(requested []string) bool {
	if len(requested) == 0 {
		return true
	}
	have := make(map[string]bool, len(a.Tags))
	for _, t := range a.Tags {
		have[t] = true
	}
	for _, t := range requested {
		if !have[t] {
			return false
		}
	}
	return true
}

// MatchesCapabilities returns true if the actor supports all required capabilities.
// An empty request matches any actor.
func (a *Actor) MatchesCapabilities(required []string) bool {
	if len(required) == 0 {
		return true
	}
	have := make(map[string]bool, len(a.Capabilities))
	for _, c := range a.Capabilities {
		have[c] = true
	}
	for _, c := range required {
		if !have[c] {
			return false
		}
	}
	return true
}
