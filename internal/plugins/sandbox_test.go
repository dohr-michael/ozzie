package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/events"
)

// --- Pattern tests ---

func TestMatchDestructivePattern(t *testing.T) {
	blocked := []struct {
		cmd    string
		reason string
	}{
		{"rm -rf /tmp/foo", "recursive remove"},
		{"rm -f file.txt", "force remove"},
		{"rm -Rf /", "recursive remove"},
		{"dd if=/dev/zero of=/dev/sda", "raw disk write (dd)"},
		{"mkfs.ext4 /dev/sdb1", "filesystem format"},
		{"fdisk /dev/sda", "partition edit"},
		{":(){ :|:& };:", "fork bomb"},
		{"echo x >/dev/sda", "raw device write"},
		{"chmod -R 777 /tmp", "recursive chmod"},
		{"chown -R root:root /", "recursive chown"},
		{"sudo apt install foo", "privilege escalation"},
		{"su root", "switch user"},
	}
	for _, tc := range blocked {
		rule := matchDestructivePattern(tc.cmd)
		if rule == nil {
			t.Errorf("expected %q to be blocked (%s), got nil", tc.cmd, tc.reason)
			continue
		}
		if rule.reason != tc.reason {
			t.Errorf("expected reason %q for %q, got %q", tc.reason, tc.cmd, rule.reason)
		}
	}

	allowed := []string{
		"ls -la",
		"echo hello",
		"cat /etc/hostname",
		"git status",
		"go build ./...",
		"rm file.txt",      // no -r or -f
		"chmod 644 foo.txt", // no -R
	}
	for _, cmd := range allowed {
		if rule := matchDestructivePattern(cmd); rule != nil {
			t.Errorf("expected %q to be allowed, got blocked: %s", cmd, rule.reason)
		}
	}
}

// --- Path extraction tests ---

func TestExtractCommandPaths(t *testing.T) {
	tests := []struct {
		cmd    string
		expect []string
	}{
		{"cat /etc/passwd", []string{"/etc/passwd"}},
		{"ls ~/Documents", []string{"~/Documents"}},
		{"cp ../../escape/file.txt .", []string{"../../escape/file.txt"}},
		{"echo hello | tee /tmp/out.txt", []string{"/tmp/out.txt"}},
		{"ls -la", nil},
		{"echo hello", nil},
	}
	for _, tc := range tests {
		paths := extractCommandPaths(tc.cmd)
		if len(paths) != len(tc.expect) {
			t.Errorf("extractCommandPaths(%q): got %v, want %v", tc.cmd, paths, tc.expect)
			continue
		}
		for i, p := range paths {
			if p != tc.expect[i] {
				t.Errorf("extractCommandPaths(%q)[%d]: got %q, want %q", tc.cmd, i, p, tc.expect[i])
			}
		}
	}
}

func TestExtractToolPaths(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		expect map[string]bool // use set because recursive order is non-deterministic
	}{
		{"top-level path", `{"path":"/etc/passwd","content":"x"}`, map[string]bool{"/etc/passwd": true}},
		{"relative path", `{"path":"./local.txt"}`, map[string]bool{"./local.txt": true}},
		{"working_dir", `{"working_dir":"/tmp","command":"ls"}`, map[string]bool{"/tmp": true}},
		{"path + working_dir", `{"path":"/a","working_dir":"/b"}`, map[string]bool{"/a": true, "/b": true}},
		{"no paths", `{"command":"ls"}`, nil},
		{"invalid json", `invalid json`, nil},
		// Recursive: git-style nested args
		{"nested path", `{"action":"status","args":{"path":"/repo/file.go"}}`, map[string]bool{"/repo/file.go": true}},
		{"nested paths array", `{"action":"add","args":{"paths":["/a","/b"]}}`, map[string]bool{"/a": true, "/b": true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			paths := extractToolPaths(tc.json)
			if len(tc.expect) == 0 {
				if len(paths) != 0 {
					t.Errorf("expected no paths, got %v", paths)
				}
				return
			}
			got := make(map[string]bool, len(paths))
			for _, p := range paths {
				got[p] = true
			}
			for expected := range tc.expect {
				if !got[expected] {
					t.Errorf("expected path %q not found in %v", expected, paths)
				}
			}
			if len(got) != len(tc.expect) {
				t.Errorf("expected %d paths, got %d: %v", len(tc.expect), len(got), paths)
			}
		})
	}
}

// --- Path validation tests ---

func TestValidatePathInWorkDir(t *testing.T) {
	workDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		allowed []string
		wantErr bool
	}{
		{"inside workdir absolute", filepath.Join(workDir, "subdir", "file.txt"), nil, false},
		{"workdir itself", workDir, nil, false},
		{"outside workdir", "/etc/passwd", nil, true},
		{"relative inside", "subdir/file.txt", nil, false},
		{"relative escape", "../../etc/passwd", nil, true},
		{"tilde treated as relative", "~/some/file", nil, false},
		{"allowed path", "/opt/tools/bin", []string{"/opt/tools"}, false},
		{"not in allowed", "/opt/other/bin", []string{"/opt/tools"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePathInWorkDir(workDir, tc.path, tc.allowed)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for path %q, got nil", tc.path)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error for path %q, got: %v", tc.path, err)
			}
		})
	}
}

func TestValidatePathInWorkDir_Symlink(t *testing.T) {
	workDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create a symlink inside workDir that points outside
	linkPath := filepath.Join(workDir, "escape-link")
	if err := os.Symlink(outsideDir, linkPath); err != nil {
		t.Skip("symlinks not supported")
	}

	// The symlink target is outside workDir — should be blocked
	err := validatePathInWorkDir(workDir, linkPath, nil)
	if err == nil {
		t.Error("expected symlink escape to be blocked")
	}
}

// --- SandboxGuard integration tests ---

// fakeTool is a minimal InvokableTool for testing.
type fakeTool struct {
	called bool
}

func (f *fakeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "fake"}, nil
}

func (f *fakeTool) InvokableRun(_ context.Context, _ string, _ ...tool.Option) (string, error) {
	f.called = true
	return "ok", nil
}

func autonomousCtx(workDir string) context.Context {
	ctx := events.WithAutonomous(context.Background())
	if workDir != "" {
		ctx = events.ContextWithWorkDir(ctx, workDir)
	}
	return ctx
}

func TestSandboxGuard_ExecBlocked(t *testing.T) {
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "cmd", sandboxExec, false, nil)

	ctx := autonomousCtx("")
	args := `{"command":"rm -rf /tmp/important"}`
	_, err := guard.InvokableRun(ctx, args)
	if err == nil {
		t.Fatal("expected destructive command to be blocked")
	}
	if inner.called {
		t.Error("inner tool should not have been called")
	}
}

func TestSandboxGuard_ExecAllowed(t *testing.T) {
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "cmd", sandboxExec, false, nil)

	ctx := autonomousCtx("")
	args := `{"command":"echo hello"}`
	result, err := guard.InvokableRun(ctx, args)
	if err != nil {
		t.Fatalf("expected safe command to pass, got: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
	if !inner.called {
		t.Error("inner tool should have been called")
	}
}

func TestSandboxGuard_ExecPathBlocked(t *testing.T) {
	workDir := t.TempDir()
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "cmd", sandboxExec, false, nil)

	ctx := autonomousCtx(workDir)
	args := `{"command":"cat /etc/passwd"}`
	_, err := guard.InvokableRun(ctx, args)
	if err == nil {
		t.Fatal("expected path outside workdir to be blocked")
	}
}

func TestSandboxGuard_ExecWorkDirOverrideBlocked(t *testing.T) {
	workDir := t.TempDir()
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "cmd", sandboxExec, false, nil)

	ctx := autonomousCtx(workDir)
	args := `{"command":"ls","working_dir":"/etc"}`
	_, err := guard.InvokableRun(ctx, args)
	if err == nil {
		t.Fatal("expected working_dir override outside workdir to be blocked")
	}
}

func TestSandboxGuard_FilesystemBlocked(t *testing.T) {
	workDir := t.TempDir()
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "write_file", sandboxFilesystem, false, nil)

	ctx := autonomousCtx(workDir)
	args := `{"path":"/etc/passwd","content":"hacked"}`
	_, err := guard.InvokableRun(ctx, args)
	if err == nil {
		t.Fatal("expected write outside workdir to be blocked")
	}
	if inner.called {
		t.Error("inner tool should not have been called")
	}
}

func TestSandboxGuard_FilesystemAllowed(t *testing.T) {
	workDir := t.TempDir()
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "write_file", sandboxFilesystem, false, nil)

	ctx := autonomousCtx(workDir)
	args := `{"path":"` + filepath.Join(workDir, "file.txt") + `","content":"ok"}`
	result, err := guard.InvokableRun(ctx, args)
	if err != nil {
		t.Fatalf("expected write inside workdir to pass, got: %v", err)
	}
	if result != "ok" || !inner.called {
		t.Error("inner tool should have been called successfully")
	}
}

func TestSandboxGuard_InteractivePassthrough(t *testing.T) {
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "cmd", sandboxExec, false, nil)

	// Non-autonomous context — everything should pass
	ctx := context.Background()
	args := `{"command":"rm -rf /"}`
	result, err := guard.InvokableRun(ctx, args)
	if err != nil {
		t.Fatalf("expected interactive mode to pass through, got: %v", err)
	}
	if result != "ok" || !inner.called {
		t.Error("inner tool should have been called")
	}
}

func TestSandboxGuard_RootCmdAlwaysBlocked(t *testing.T) {
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "root_cmd", sandboxExec, true, nil)

	ctx := autonomousCtx("")
	args := `{"command":"ls"}`
	_, err := guard.InvokableRun(ctx, args)
	if err == nil {
		t.Fatal("expected root_cmd to be blocked in autonomous mode")
	}
	if inner.called {
		t.Error("inner tool should not have been called")
	}
}

func TestSandboxGuard_FilesystemNoWorkDir(t *testing.T) {
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "write_file", sandboxFilesystem, false, nil)

	// Autonomous but no workDir — only denylist applies (filesystem has none), so pass
	ctx := autonomousCtx("")
	args := `{"path":"/etc/passwd","content":"ok"}`
	result, err := guard.InvokableRun(ctx, args)
	if err != nil {
		t.Fatalf("expected no workdir to skip path jail, got: %v", err)
	}
	if result != "ok" || !inner.called {
		t.Error("inner tool should have been called")
	}
}

func TestSandboxGuard_AllowedPaths(t *testing.T) {
	workDir := t.TempDir()
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "read_file", sandboxFilesystem, false, []string{"/opt/shared"})

	ctx := autonomousCtx(workDir)
	args := `{"path":"/opt/shared/data.txt"}`
	result, err := guard.InvokableRun(ctx, args)
	if err != nil {
		t.Fatalf("expected allowed path to pass, got: %v", err)
	}
	if result != "ok" || !inner.called {
		t.Error("inner tool should have been called")
	}
}

// --- Fix 1: structured exec tools (git) ---

func TestSandboxGuard_GitNoCommandField(t *testing.T) {
	// git tool has action+args, not command — should pass denylist (no command to check)
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "git", sandboxExec, false, nil)

	ctx := autonomousCtx("")
	args := `{"action":"status","args":{}}`
	result, err := guard.InvokableRun(ctx, args)
	if err != nil {
		t.Fatalf("expected git tool without command to pass, got: %v", err)
	}
	if result != "ok" || !inner.called {
		t.Error("inner tool should have been called")
	}
}

func TestSandboxGuard_GitPathOutsideWorkDir(t *testing.T) {
	workDir := t.TempDir()
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "git", sandboxExec, false, nil)

	ctx := autonomousCtx(workDir)
	// git add with a path outside workDir — should be blocked by path jail
	args := `{"action":"add","args":{"paths":["/etc/passwd"]}}`
	_, err := guard.InvokableRun(ctx, args)
	if err == nil {
		t.Fatal("expected git path outside workdir to be blocked")
	}
	if inner.called {
		t.Error("inner tool should not have been called")
	}
}

func TestSandboxGuard_GitPathInsideWorkDir(t *testing.T) {
	workDir := t.TempDir()
	inner := &fakeTool{}
	guard := WrapSandbox(inner, "git", sandboxExec, false, nil)

	ctx := autonomousCtx(workDir)
	args := `{"action":"add","args":{"paths":["src/main.go"]}}`
	result, err := guard.InvokableRun(ctx, args)
	if err != nil {
		t.Fatalf("expected relative git path to pass, got: %v", err)
	}
	if result != "ok" || !inner.called {
		t.Error("inner tool should have been called")
	}
}

// --- Fix 3: WrapRegistrySandbox skips read-only FS ---

func TestWrapRegistrySandbox_SkipsReadOnly(t *testing.T) {
	bus := events.NewBus(16)
	defer bus.Close()
	registry := NewToolRegistry(bus)

	readTool := &fakeTool{}
	writeTool := &fakeTool{}

	// read_file: Filesystem with ReadOnly=true — should NOT be wrapped
	_ = registry.RegisterNative("read_file", readTool, &PluginManifest{
		Name:     "read_file",
		Provider: "native",
		Capabilities: CapabilitySet{
			Filesystem: &FSCapability{ReadOnly: true},
		},
		Tools: []ToolSpec{{Name: "read_file"}},
	})

	// write_file: Filesystem without ReadOnly — should be wrapped
	_ = registry.RegisterNative("write_file", writeTool, &PluginManifest{
		Name:      "write_file",
		Provider:  "native",
		Dangerous: true,
		Capabilities: CapabilitySet{
			Filesystem: &FSCapability{},
		},
		Tools: []ToolSpec{{Name: "write_file", Dangerous: true}},
	})

	WrapRegistrySandbox(registry, nil)

	// read_file should NOT be a SandboxGuard
	if _, ok := registry.tools["read_file"].(*SandboxGuard); ok {
		t.Error("read_file (read-only) should not be wrapped by SandboxGuard")
	}

	// write_file should be a SandboxGuard
	if _, ok := registry.tools["write_file"].(*SandboxGuard); !ok {
		t.Error("write_file should be wrapped by SandboxGuard")
	}
}
