// Package policy defines session policies and identity-to-policy pairing.
package policy

// Policy defines what a session can do.
type Policy struct {
	Name          string   `json:"name"`
	SessionMode   string   `json:"session_mode"`   // "persistent" | "ephemeral" | "per-request"
	AllowedSkills []string `json:"allowed_skills"`  // nil = all
	AllowedTools  []string `json:"allowed_tools"`   // nil = all
	DeniedTools   []string `json:"denied_tools"`
	ApprovalMode  string   `json:"approval_mode"`   // "sync" | "async" | "none"
	ClientFacing  bool     `json:"client_facing"`   // inject Persona
	MaxConcurrent int      `json:"max_concurrent"`
}

// AdminPolicy returns the full-access policy for admin/owner sessions.
func AdminPolicy() Policy {
	return Policy{
		Name:          "admin",
		SessionMode:   "persistent",
		AllowedSkills: nil, // all
		AllowedTools:  nil, // all
		ApprovalMode:  "sync",
		ClientFacing:  true,
		MaxConcurrent: 4,
	}
}

// SupportPolicy returns a scoped policy for support/help-desk sessions.
func SupportPolicy() Policy {
	return Policy{
		Name:          "support",
		SessionMode:   "ephemeral",
		AllowedSkills: nil, // all
		AllowedTools:  nil, // all (dangerous tools denied below)
		DeniedTools:   []string{"run_command", "write_file", "edit_file"},
		ApprovalMode:  "none",
		ClientFacing:  true,
		MaxConcurrent: 2,
	}
}

// ExecutorPolicy returns a policy for autonomous task execution (no user interaction).
func ExecutorPolicy() Policy {
	return Policy{
		Name:          "executor",
		SessionMode:   "per-request",
		AllowedSkills: nil, // all
		AllowedTools:  nil, // all
		ApprovalMode:  "none",
		ClientFacing:  false,
		MaxConcurrent: 2,
	}
}

// ReadonlyPolicy returns a policy with no tool access (observation only).
func ReadonlyPolicy() Policy {
	return Policy{
		Name:          "readonly",
		SessionMode:   "ephemeral",
		AllowedSkills: nil,
		AllowedTools:  []string{}, // none
		ApprovalMode:  "none",
		ClientFacing:  true,
		MaxConcurrent: 1,
	}
}

// predefinedPolicies returns the built-in policies keyed by name.
func predefinedPolicies() map[string]Policy {
	return map[string]Policy{
		"admin":    AdminPolicy(),
		"support":  SupportPolicy(),
		"executor": ExecutorPolicy(),
		"readonly": ReadonlyPolicy(),
	}
}
