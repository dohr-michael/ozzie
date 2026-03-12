package policy

import "testing"

func TestResolveDefaults(t *testing.T) {
	r := NewPolicyResolver(nil)
	p, ok := r.Resolve("admin")
	if !ok {
		t.Fatal("admin policy not found")
	}
	if p.Name != "admin" {
		t.Errorf("name = %q, want admin", p.Name)
	}
	if !p.ClientFacing {
		t.Error("default admin should be client-facing")
	}
}

func TestResolveWithOverride(t *testing.T) {
	clientFacing := false
	r := NewPolicyResolver(map[string]Override{
		"admin": {
			MaxConcurrent: 10,
			ClientFacing:  &clientFacing,
			DeniedTools:   []string{"run_command"},
		},
	})
	p, ok := r.Resolve("admin")
	if !ok {
		t.Fatal("admin policy not found")
	}
	if p.MaxConcurrent != 10 {
		t.Errorf("max_concurrent = %d, want 10", p.MaxConcurrent)
	}
	if p.ClientFacing {
		t.Error("override should have set client_facing=false")
	}
	if len(p.DeniedTools) != 1 || p.DeniedTools[0] != "run_command" {
		t.Errorf("denied_tools = %v, want [run_command]", p.DeniedTools)
	}
	// Non-overridden fields keep defaults
	if p.SessionMode != "persistent" {
		t.Errorf("session_mode = %q, want persistent (kept from default)", p.SessionMode)
	}
}

func TestResolveUnknown(t *testing.T) {
	r := NewPolicyResolver(nil)
	_, ok := r.Resolve("nonexistent")
	if ok {
		t.Error("expected unknown policy to return false")
	}
}

func TestOverrideUnknownPolicyIgnored(t *testing.T) {
	r := NewPolicyResolver(map[string]Override{
		"custom": {MaxConcurrent: 5},
	})
	names := r.Names()
	for _, n := range names {
		if n == "custom" {
			t.Error("unknown policy override should be ignored")
		}
	}
}

func TestNames(t *testing.T) {
	r := NewPolicyResolver(nil)
	names := r.Names()
	if len(names) != 4 {
		t.Fatalf("expected 4 names, got %d: %v", len(names), names)
	}
	// Should be sorted
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted: %v", names)
			break
		}
	}
}
