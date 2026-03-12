package agent

import (
	"sort"
	"sync"
	"testing"
)

func TestToolSet_CoreAlwaysActive(t *testing.T) {
	ts := NewToolSet([]string{"cmd", "read_file"}, []string{"cmd", "read_file", "search", "git"})

	names := ts.ActiveToolNames("s1")
	if len(names) != 2 {
		t.Fatalf("ActiveToolNames len = %d, want 2", len(names))
	}
	// Should be sorted
	if names[0] != "cmd" || names[1] != "read_file" {
		t.Errorf("ActiveToolNames = %v, want [cmd read_file]", names)
	}
}

func TestToolSet_ActivateKnown(t *testing.T) {
	ts := NewToolSet([]string{"cmd"}, []string{"cmd", "search", "git"})

	ok := ts.Activate("s1", "search")
	if !ok {
		t.Fatal("Activate(search) returned false")
	}

	names := ts.ActiveToolNames("s1")
	expected := []string{"cmd", "search"}
	if len(names) != len(expected) {
		t.Fatalf("ActiveToolNames len = %d, want %d", len(names), len(expected))
	}
	for i, n := range expected {
		if names[i] != n {
			t.Errorf("ActiveToolNames[%d] = %q, want %q", i, names[i], n)
		}
	}
}

func TestToolSet_ActivateUnknown(t *testing.T) {
	ts := NewToolSet([]string{"cmd"}, []string{"cmd", "search"})

	ok := ts.Activate("s1", "nonexistent")
	if ok {
		t.Fatal("Activate(nonexistent) returned true")
	}
}

func TestToolSet_ActivateIdempotent(t *testing.T) {
	ts := NewToolSet([]string{"cmd"}, []string{"cmd", "search"})

	ts.Activate("s1", "search")
	ts.Activate("s1", "search")

	names := ts.ActiveToolNames("s1")
	if len(names) != 2 {
		t.Fatalf("ActiveToolNames len = %d, want 2 after double activate", len(names))
	}
}

func TestToolSet_SessionIsolation(t *testing.T) {
	ts := NewToolSet([]string{"cmd"}, []string{"cmd", "search", "git"})

	ts.Activate("s1", "search")
	ts.Activate("s2", "git")

	s1Names := ts.ActiveToolNames("s1")
	s2Names := ts.ActiveToolNames("s2")

	if len(s1Names) != 2 {
		t.Fatalf("s1 ActiveToolNames len = %d, want 2", len(s1Names))
	}
	if len(s2Names) != 2 {
		t.Fatalf("s2 ActiveToolNames len = %d, want 2", len(s2Names))
	}

	if !ts.IsActive("s1", "search") {
		t.Error("search should be active for s1")
	}
	if ts.IsActive("s1", "git") {
		t.Error("git should NOT be active for s1")
	}
	if ts.IsActive("s2", "search") {
		t.Error("search should NOT be active for s2")
	}
	if !ts.IsActive("s2", "git") {
		t.Error("git should be active for s2")
	}
}

func TestToolSet_TurnFlags(t *testing.T) {
	ts := NewToolSet([]string{"cmd"}, []string{"cmd", "search"})

	ts.ResetTurnFlag("s1")
	if ts.ActivatedDuringTurn("s1") {
		t.Error("ActivatedDuringTurn should be false after reset")
	}

	ts.Activate("s1", "search")
	if !ts.ActivatedDuringTurn("s1") {
		t.Error("ActivatedDuringTurn should be true after activate")
	}

	ts.ResetTurnFlag("s1")
	if ts.ActivatedDuringTurn("s1") {
		t.Error("ActivatedDuringTurn should be false after second reset")
	}
}

func TestToolSet_HasInactiveTools(t *testing.T) {
	ts := NewToolSet([]string{"cmd"}, []string{"cmd", "search", "git"})

	if !ts.HasInactiveTools("s1") {
		t.Error("HasInactiveTools should be true when not all tools are active")
	}

	ts.Activate("s1", "search")
	ts.Activate("s1", "git")

	if ts.HasInactiveTools("s1") {
		t.Error("HasInactiveTools should be false when all tools are active")
	}
}

func TestToolSet_IsKnown(t *testing.T) {
	ts := NewToolSet([]string{"cmd"}, []string{"cmd", "search"})

	if !ts.IsKnown("cmd") {
		t.Error("IsKnown(cmd) = false")
	}
	if !ts.IsKnown("search") {
		t.Error("IsKnown(search) = false")
	}
	if ts.IsKnown("nonexistent") {
		t.Error("IsKnown(nonexistent) = true")
	}
}

func TestToolSet_Cleanup(t *testing.T) {
	ts := NewToolSet([]string{"cmd"}, []string{"cmd", "search"})

	ts.Activate("s1", "search")
	ts.Cleanup("s1")

	names := ts.ActiveToolNames("s1")
	if len(names) != 1 {
		t.Fatalf("after cleanup, ActiveToolNames len = %d, want 1 (core only)", len(names))
	}
	if names[0] != "cmd" {
		t.Errorf("after cleanup, ActiveToolNames = %v, want [cmd]", names)
	}
	if ts.ActivatedDuringTurn("s1") {
		t.Error("after cleanup, ActivatedDuringTurn should be false")
	}
}

func TestToolSet_CoreIsAlwaysActive(t *testing.T) {
	ts := NewToolSet([]string{"cmd"}, []string{"cmd", "search"})

	if !ts.IsActive("s1", "cmd") {
		t.Error("core tool cmd should always be active")
	}
}

func TestToolSet_Concurrent(t *testing.T) {
	ts := NewToolSet([]string{"cmd"}, []string{"cmd", "search", "git", "write_file", "read_file"})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sid := "s1"
			if i%2 == 0 {
				sid = "s2"
			}
			ts.Activate(sid, "search")
			ts.Activate(sid, "git")
			ts.ActiveToolNames(sid)
			ts.IsActive(sid, "search")
			ts.HasInactiveTools(sid)
			ts.ResetTurnFlag(sid)
			ts.ActivatedDuringTurn(sid)
			ts.IsKnown("search")
		}(i)
	}
	wg.Wait()

	// Sanity check after concurrent access
	names := ts.ActiveToolNames("s1")
	sort.Strings(names)
	if len(names) < 1 {
		t.Error("expected at least core tools after concurrent access")
	}
}
