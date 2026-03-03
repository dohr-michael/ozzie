package plugins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk/filesystem"

	"github.com/dohr-michael/ozzie/internal/events"
)

func ctxWithWorkDir(dir string) context.Context {
	return events.ContextWithWorkDir(context.Background(), dir)
}

func TestOzzieBackend_LsInfo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)

	b := NewOzzieBackend(nil)
	files, err := b.LsInfo(ctxWithWorkDir(dir), &filesystem.LsInfoRequest{Path: dir})
	if err != nil {
		t.Fatalf("LsInfo: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 entries, got %d", len(files))
	}
}

func TestOzzieBackend_Read(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0o644)

	b := NewOzzieBackend(nil)
	content, err := b.Read(ctxWithWorkDir(dir), &filesystem.ReadRequest{
		FilePath: path,
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(content, "line1") {
		t.Errorf("expected content to contain 'line1', got %q", content)
	}
}

func TestOzzieBackend_Read_OffsetLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("a\nb\nc\nd\ne\n"), 0o644)

	b := NewOzzieBackend(nil)
	content, err := b.Read(ctxWithWorkDir(dir), &filesystem.ReadRequest{
		FilePath: path,
		Offset:   1,
		Limit:    2,
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(content, "b") {
		t.Errorf("expected content to contain 'b', got %q", content)
	}
}

func TestOzzieBackend_GrepRaw(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world\nfoo bar\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\nfunc hello() {}\n"), 0o644)

	b := NewOzzieBackend(nil)
	matches, err := b.GrepRaw(ctxWithWorkDir(dir), &filesystem.GrepRequest{
		Pattern: "hello",
		Path:    dir,
	})
	if err != nil {
		t.Fatalf("GrepRaw: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}
}

func TestOzzieBackend_GrepRaw_WithGlob(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("match here\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("match here too\n"), 0o644)

	b := NewOzzieBackend(nil)
	matches, err := b.GrepRaw(ctxWithWorkDir(dir), &filesystem.GrepRequest{
		Pattern: "match",
		Path:    dir,
		Glob:    "*.txt",
	})
	if err != nil {
		t.Fatalf("GrepRaw: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
}

func TestOzzieBackend_GlobInfo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0o644)

	b := NewOzzieBackend(nil)
	files, err := b.GlobInfo(ctxWithWorkDir(dir), &filesystem.GlobInfoRequest{
		Pattern: "*.go",
		Path:    dir,
	})
	if err != nil {
		t.Fatalf("GlobInfo: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestOzzieBackend_Write(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new", "file.txt")

	b := NewOzzieBackend(nil)
	err := b.Write(ctxWithWorkDir(dir), &filesystem.WriteRequest{
		FilePath: path,
		Content:  "hello",
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

func TestOzzieBackend_Edit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	os.WriteFile(path, []byte("foo bar baz"), 0o644)

	b := NewOzzieBackend(nil)
	err := b.Edit(ctxWithWorkDir(dir), &filesystem.EditRequest{
		FilePath:  path,
		OldString: "bar",
		NewString: "qux",
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "foo qux baz" {
		t.Errorf("expected 'foo qux baz', got %q", string(data))
	}
}

func TestOzzieBackend_Edit_MultipleOccurrences(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	os.WriteFile(path, []byte("foo foo foo"), 0o644)

	b := NewOzzieBackend(nil)
	err := b.Edit(ctxWithWorkDir(dir), &filesystem.EditRequest{
		FilePath:  path,
		OldString: "foo",
		NewString: "bar",
	})
	if err == nil {
		t.Fatal("expected error for multiple occurrences without replace_all")
	}

	// With replace_all
	err = b.Edit(ctxWithWorkDir(dir), &filesystem.EditRequest{
		FilePath:   path,
		OldString:  "foo",
		NewString:  "bar",
		ReplaceAll: true,
	})
	if err != nil {
		t.Fatalf("Edit with replace_all: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "bar bar bar" {
		t.Errorf("expected 'bar bar bar', got %q", string(data))
	}
}

// --- Write guard tests ---

func TestWriteGuard_AutonomousBlocked(t *testing.T) {
	workDir := t.TempDir()
	outsideDir := t.TempDir()

	b := NewOzzieBackend(nil)
	err := b.Write(autonomousCtx(workDir), &filesystem.WriteRequest{
		FilePath: filepath.Join(outsideDir, "evil.txt"),
		Content:  "should be blocked",
	})
	if err == nil {
		t.Fatal("expected write outside workDir to be blocked in autonomous mode")
	}
	if !strings.Contains(err.Error(), "write blocked") {
		t.Errorf("expected 'write blocked' error, got: %v", err)
	}
}

func TestWriteGuard_AutonomousAllowedWorkDir(t *testing.T) {
	workDir := t.TempDir()

	b := NewOzzieBackend(nil)
	err := b.Write(autonomousCtx(workDir), &filesystem.WriteRequest{
		FilePath: filepath.Join(workDir, "ok.txt"),
		Content:  "allowed",
	})
	if err != nil {
		t.Fatalf("write inside workDir should be allowed: %v", err)
	}
}

func TestWriteGuard_AutonomousAllowedTmpDir(t *testing.T) {
	workDir := t.TempDir()
	tmpDir := t.TempDir()

	b := NewOzzieBackend(nil, tmpDir)
	err := b.Write(autonomousCtx(workDir), &filesystem.WriteRequest{
		FilePath: filepath.Join(tmpDir, "scratch.txt"),
		Content:  "tmp write",
	})
	if err != nil {
		t.Fatalf("write inside tmpDir should be allowed: %v", err)
	}
}

func TestWriteGuard_InteractiveInsideSandbox(t *testing.T) {
	workDir := t.TempDir()
	ctx := ctxWithWorkDir(workDir)

	b := NewOzzieBackend(nil)
	err := b.Write(ctx, &filesystem.WriteRequest{
		FilePath: filepath.Join(workDir, "ok.txt"),
		Content:  "inside sandbox — always allowed",
	})
	if err != nil {
		t.Fatalf("interactive write inside sandbox should be allowed: %v", err)
	}
}

func TestWriteGuard_InteractiveOutsideSandboxConfirmed(t *testing.T) {
	workDir := t.TempDir()
	outsideDir := t.TempDir()
	ctx := ctxWithWorkDir(workDir)

	bus := events.NewBus(16)
	defer bus.Close()

	b := NewOzzieBackend(bus)

	// Simulate user approval in background
	ch, unsub := bus.SubscribeChan(1, events.EventPromptRequest)
	go func() {
		defer unsub()
		evt := <-ch
		payload, ok := events.GetPromptRequestPayload(evt)
		if !ok {
			return
		}
		bus.Publish(events.NewTypedEvent(events.SourceWS, events.PromptResponsePayload{
			Token: payload.Token,
		}))
	}()

	err := b.Write(ctx, &filesystem.WriteRequest{
		FilePath: filepath.Join(outsideDir, "ok.txt"),
		Content:  "user approved",
	})
	if err != nil {
		t.Fatalf("interactive write outside sandbox should succeed after confirmation: %v", err)
	}
}

func TestWriteGuard_InteractiveOutsideSandboxDenied(t *testing.T) {
	workDir := t.TempDir()
	outsideDir := t.TempDir()
	ctx := ctxWithWorkDir(workDir)

	bus := events.NewBus(16)
	defer bus.Close()

	b := NewOzzieBackend(bus)

	// Simulate user denial in background
	ch, unsub := bus.SubscribeChan(1, events.EventPromptRequest)
	go func() {
		defer unsub()
		evt := <-ch
		payload, ok := events.GetPromptRequestPayload(evt)
		if !ok {
			return
		}
		bus.Publish(events.NewTypedEvent(events.SourceWS, events.PromptResponsePayload{
			Token:     payload.Token,
			Cancelled: true,
		}))
	}()

	err := b.Write(ctx, &filesystem.WriteRequest{
		FilePath: filepath.Join(outsideDir, "denied.txt"),
		Content:  "should be denied",
	})
	if err == nil {
		t.Fatal("expected write to be denied by user")
	}
	if !strings.Contains(err.Error(), "write denied") {
		t.Errorf("expected 'write denied' error, got: %v", err)
	}
}

func TestEditGuard_AutonomousBlocked(t *testing.T) {
	workDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create the file outside workDir
	target := filepath.Join(outsideDir, "target.txt")
	os.WriteFile(target, []byte("old content"), 0o644)

	b := NewOzzieBackend(nil)
	err := b.Edit(autonomousCtx(workDir), &filesystem.EditRequest{
		FilePath:  target,
		OldString: "old",
		NewString: "new",
	})
	if err == nil {
		t.Fatal("expected edit outside workDir to be blocked in autonomous mode")
	}
	if !strings.Contains(err.Error(), "write blocked") {
		t.Errorf("expected 'write blocked' error, got: %v", err)
	}
}

func TestOzzieBackend_GlobInfo_Recursive(t *testing.T) {
	dir := t.TempDir()
	// Create nested structure
	os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755)
	os.WriteFile(filepath.Join(dir, "top.go"), []byte("package top"), 0o644)
	os.WriteFile(filepath.Join(dir, "a", "mid.go"), []byte("package mid"), 0o644)
	os.WriteFile(filepath.Join(dir, "a", "b", "deep.go"), []byte("package deep"), 0o644)
	os.WriteFile(filepath.Join(dir, "a", "b", "deep.txt"), []byte("not go"), 0o644)

	b := NewOzzieBackend(nil)
	files, err := b.GlobInfo(ctxWithWorkDir(dir), &filesystem.GlobInfoRequest{
		Pattern: "**/*.go",
		Path:    dir,
	})
	if err != nil {
		t.Fatalf("GlobInfo recursive: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("expected 3 .go files across all levels, got %d", len(files))
		for _, f := range files {
			t.Logf("  %s", f.Path)
		}
	}
}
