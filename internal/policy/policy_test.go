package policy

import "testing"

func TestAdminPolicyDefaults(t *testing.T) {
	p := AdminPolicy()
	if p.Name != "admin" {
		t.Errorf("name = %q, want admin", p.Name)
	}
	if p.SessionMode != "persistent" {
		t.Errorf("session_mode = %q, want persistent", p.SessionMode)
	}
	if !p.ClientFacing {
		t.Error("admin should be client-facing")
	}
	if p.AllowedTools != nil {
		t.Error("admin should allow all tools (nil)")
	}
	if p.ApprovalMode != "sync" {
		t.Errorf("approval_mode = %q, want sync", p.ApprovalMode)
	}
}

func TestSupportPolicyDeniedTools(t *testing.T) {
	p := SupportPolicy()
	if len(p.DeniedTools) == 0 {
		t.Error("support policy should deny some tools")
	}
	if p.ApprovalMode != "none" {
		t.Errorf("approval_mode = %q, want none", p.ApprovalMode)
	}
}

func TestExecutorPolicyNotClientFacing(t *testing.T) {
	p := ExecutorPolicy()
	if p.ClientFacing {
		t.Error("executor should NOT be client-facing")
	}
	if p.SessionMode != "per-request" {
		t.Errorf("session_mode = %q, want per-request", p.SessionMode)
	}
}

func TestReadonlyPolicyNoTools(t *testing.T) {
	p := ReadonlyPolicy()
	if p.AllowedTools == nil {
		t.Error("readonly should have explicit empty tools (not nil)")
	}
	if len(p.AllowedTools) != 0 {
		t.Errorf("readonly should have 0 allowed tools, got %d", len(p.AllowedTools))
	}
}

func TestPredefinedPoliciesCount(t *testing.T) {
	policies := predefinedPolicies()
	if len(policies) != 4 {
		t.Errorf("expected 4 predefined policies, got %d", len(policies))
	}
	for _, name := range []string{"admin", "support", "executor", "readonly"} {
		if _, ok := policies[name]; !ok {
			t.Errorf("missing predefined policy %q", name)
		}
	}
}
