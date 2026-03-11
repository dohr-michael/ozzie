package policy

import "slices"

// Override allows customizing a predefined policy via config.
// Zero values are ignored (keep the default).
type Override struct {
	AllowedSkills []string `json:"allowed_skills,omitempty"`
	AllowedTools  []string `json:"allowed_tools,omitempty"`
	DeniedTools   []string `json:"denied_tools,omitempty"`
	ApprovalMode  string   `json:"approval_mode,omitempty"`
	ClientFacing  *bool    `json:"client_facing,omitempty"`
	MaxConcurrent int      `json:"max_concurrent,omitempty"`
}

// PolicyResolver resolves a policy by name, merging config overrides with defaults.
type PolicyResolver struct {
	policies map[string]Policy
}

// NewPolicyResolver creates a resolver with the predefined policies, then applies overrides.
func NewPolicyResolver(overrides map[string]Override) *PolicyResolver {
	policies := predefinedPolicies()
	for name, ov := range overrides {
		base, ok := policies[name]
		if !ok {
			continue // skip overrides for unknown policy names
		}
		if len(ov.AllowedSkills) > 0 {
			base.AllowedSkills = ov.AllowedSkills
		}
		if len(ov.AllowedTools) > 0 {
			base.AllowedTools = ov.AllowedTools
		}
		if len(ov.DeniedTools) > 0 {
			base.DeniedTools = ov.DeniedTools
		}
		if ov.ApprovalMode != "" {
			base.ApprovalMode = ov.ApprovalMode
		}
		if ov.ClientFacing != nil {
			base.ClientFacing = *ov.ClientFacing
		}
		if ov.MaxConcurrent > 0 {
			base.MaxConcurrent = ov.MaxConcurrent
		}
		policies[name] = base
	}
	return &PolicyResolver{policies: policies}
}

// Resolve returns the policy for the given name, or false if unknown.
func (r *PolicyResolver) Resolve(name string) (Policy, bool) {
	p, ok := r.policies[name]
	return p, ok
}

// Names returns all known policy names in sorted order.
func (r *PolicyResolver) Names() []string {
	names := make([]string, 0, len(r.policies))
	for n := range r.policies {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}
