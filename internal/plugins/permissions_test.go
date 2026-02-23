package plugins

import "testing"

func TestToolPermissions_GlobalAllowed(t *testing.T) {
	tp := NewToolPermissions([]string{"cmd", "git"})

	if !tp.IsAllowed("sess1", "cmd") {
		t.Error("expected cmd to be globally allowed")
	}
	if !tp.IsAllowed("sess1", "git") {
		t.Error("expected git to be globally allowed")
	}
	if tp.IsAllowed("sess1", "run_command") {
		t.Error("expected run_command to NOT be globally allowed")
	}
}

func TestToolPermissions_SessionAllowed(t *testing.T) {
	tp := NewToolPermissions(nil)

	if tp.IsAllowed("sess1", "run_command") {
		t.Error("expected run_command to be denied before approval")
	}

	tp.AllowForSession("sess1", "run_command")

	if !tp.IsAllowed("sess1", "run_command") {
		t.Error("expected run_command to be allowed after session approval")
	}
	if tp.IsAllowed("sess2", "run_command") {
		t.Error("expected run_command to be denied for other sessions")
	}
}

func TestToolPermissions_AcceptAll(t *testing.T) {
	tp := NewToolPermissions(nil)

	tp.AllowAllForSession("sess1")

	if !tp.IsAllowed("sess1", "run_command") {
		t.Error("expected any tool to be allowed in accept-all mode")
	}
	if !tp.IsAllowed("sess1", "root_cmd") {
		t.Error("expected any tool to be allowed in accept-all mode")
	}
	if !tp.IsSessionAcceptAll("sess1") {
		t.Error("expected IsSessionAcceptAll to return true")
	}
	if tp.IsSessionAcceptAll("sess2") {
		t.Error("expected IsSessionAcceptAll to return false for other session")
	}
}

func TestToolPermissions_Cleanup(t *testing.T) {
	tp := NewToolPermissions(nil)

	tp.AllowForSession("sess1", "cmd")
	tp.AllowAllForSession("sess1")

	if !tp.IsAllowed("sess1", "cmd") {
		t.Error("expected cmd to be allowed before cleanup")
	}

	tp.CleanupSession("sess1")

	if tp.IsAllowed("sess1", "cmd") {
		t.Error("expected cmd to be denied after cleanup")
	}
	if tp.IsSessionAcceptAll("sess1") {
		t.Error("expected accept-all to be cleared after cleanup")
	}
}
