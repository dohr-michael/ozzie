package editor_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dohr-michael/ozzie/pkg/editor"
)

func setup(t *testing.T) (context.Context, *editor.Editor, string) {
	t.Helper()
	dir := t.TempDir()
	e := editor.New(editor.LocalBackend{})
	return context.Background(), e, dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- View tests ---

func TestView_FullFile(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "hello.txt")
	writeFile(t, path, "line1\nline2\nline3")

	out, err := e.View(ctx, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "line1") || !strings.Contains(out, "line3") {
		t.Fatalf("expected all lines, got:\n%s", out)
	}
}

func TestView_Range(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "range.txt")
	writeFile(t, path, "a\nb\nc\nd\ne")

	out, err := e.View(ctx, path, []int{2, 4})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "b") || !strings.Contains(out, "d") {
		t.Fatalf("expected lines 2-4, got:\n%s", out)
	}
	if strings.Contains(out, "\ta\n") {
		t.Fatalf("should not contain line 1 content")
	}
}

func TestView_Directory(t *testing.T) {
	ctx, e, dir := setup(t)
	writeFile(t, filepath.Join(dir, "sub", "file.txt"), "x")

	out, err := e.View(ctx, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sub") {
		t.Fatalf("expected directory listing with 'sub', got:\n%s", out)
	}
}

func TestView_NotFound(t *testing.T) {
	ctx, e, dir := setup(t)
	_, err := e.View(ctx, filepath.Join(dir, "nope.txt"), nil)
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestView_InvalidRange(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "small.txt")
	writeFile(t, path, "only")

	_, err := e.View(ctx, path, []int{5, 3})
	if err == nil {
		t.Fatal("expected error for invalid range")
	}
}

// --- Create tests ---

func TestCreate_OK(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "new.txt")

	out, err := e.Create(ctx, path, "hello\nworld")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected content in output, got:\n%s", out)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello\nworld" {
		t.Fatalf("unexpected file content: %q", data)
	}
}

func TestCreate_AlreadyExists(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "exists.txt")
	writeFile(t, path, "old")

	_, err := e.Create(ctx, path, "new")
	if err == nil {
		t.Fatal("expected error for existing file")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreate_NestedDirs(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "a", "b", "c.txt")

	_, err := e.Create(ctx, path, "deep")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "deep" {
		t.Fatalf("unexpected content: %q", data)
	}
}

// --- StrReplace tests ---

func TestStrReplace_OK(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "replace.txt")
	writeFile(t, path, "foo bar baz")

	out, err := e.StrReplace(ctx, path, "bar", "qux")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "qux") {
		t.Fatalf("expected 'qux' in snippet, got:\n%s", out)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "foo qux baz" {
		t.Fatalf("unexpected content: %q", data)
	}
}

func TestStrReplace_MultiOccurrence(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "multi.txt")
	writeFile(t, path, "aaa bbb aaa")

	_, err := e.StrReplace(ctx, path, "aaa", "ccc")
	if err == nil {
		t.Fatal("expected error for multiple occurrences")
	}
	if !strings.Contains(err.Error(), "2 times") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStrReplace_NotFound(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "nf.txt")
	writeFile(t, path, "hello world")

	_, err := e.StrReplace(ctx, path, "xyz", "abc")
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Insert tests ---

func TestInsert_Middle(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "insert.txt")
	writeFile(t, path, "line1\nline2\nline3")

	out, err := e.Insert(ctx, path, 1, "inserted")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "inserted") {
		t.Fatalf("expected 'inserted' in output, got:\n%s", out)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "line1\ninserted\nline2\nline3" {
		t.Fatalf("unexpected content: %q", data)
	}
}

func TestInsert_Beginning(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "begin.txt")
	writeFile(t, path, "a\nb")

	_, err := e.Insert(ctx, path, 0, "top")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "top\na\nb" {
		t.Fatalf("unexpected content: %q", data)
	}
}

func TestInsert_End(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "end.txt")
	writeFile(t, path, "a\nb")

	_, err := e.Insert(ctx, path, 2, "bottom")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "a\nb\nbottom" {
		t.Fatalf("unexpected content: %q", data)
	}
}

func TestInsert_OutOfRange(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "oor.txt")
	writeFile(t, path, "x")

	_, err := e.Insert(ctx, path, 5, "nope")
	if err == nil {
		t.Fatal("expected error for out of range")
	}
}

// --- UndoEdit tests ---

func TestUndoEdit_OK(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "undo.txt")
	writeFile(t, path, "original")

	_, err := e.StrReplace(ctx, path, "original", "modified")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "modified" {
		t.Fatalf("expected 'modified', got %q", data)
	}

	_, err = e.UndoEdit(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "original" {
		t.Fatalf("expected 'original' after undo, got %q", data)
	}
}

func TestUndoEdit_NoHistory(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "nohistory.txt")
	writeFile(t, path, "content")

	_, err := e.UndoEdit(ctx, path)
	if err == nil {
		t.Fatal("expected error for no history")
	}
	if !strings.Contains(err.Error(), "no edit history") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUndoEdit_MultipleUndos(t *testing.T) {
	ctx, e, dir := setup(t)
	path := filepath.Join(dir, "multi_undo.txt")
	writeFile(t, path, "v1")

	if _, err := e.StrReplace(ctx, path, "v1", "v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.StrReplace(ctx, path, "v2", "v3"); err != nil {
		t.Fatal(err)
	}

	// Undo to v2
	if _, err := e.UndoEdit(ctx, path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "v2" {
		t.Fatalf("expected 'v2', got %q", data)
	}

	// Undo to v1
	if _, err := e.UndoEdit(ctx, path); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "v1" {
		t.Fatalf("expected 'v1', got %q", data)
	}

	// No more history
	if _, err := e.UndoEdit(ctx, path); err == nil {
		t.Fatal("expected error for empty history")
	}
}
