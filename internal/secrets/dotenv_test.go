package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetEntry_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if err := SetEntry(path, "API_KEY", "secret123"); err != nil {
		t.Fatalf("SetEntry: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !strings.Contains(string(data), "API_KEY=secret123") {
		t.Errorf("expected API_KEY=secret123, got:\n%s", data)
	}
}

func TestSetEntry_UpdateExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	initial := "# comment\nFOO=bar\nBAZ=qux\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := SetEntry(path, "FOO", "updated"); err != nil {
		t.Fatalf("SetEntry: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "FOO=updated") {
		t.Errorf("expected FOO=updated, got:\n%s", content)
	}
	if !strings.Contains(content, "# comment") {
		t.Error("comment was lost")
	}
	if !strings.Contains(content, "BAZ=qux") {
		t.Error("other entries were lost")
	}
}

func TestSetEntry_AppendsNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	initial := "EXISTING=value\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := SetEntry(path, "NEW_KEY", "new_value"); err != nil {
		t.Fatalf("SetEntry: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "EXISTING=value") {
		t.Error("existing entry was lost")
	}
	if !strings.Contains(content, "NEW_KEY=new_value") {
		t.Errorf("new entry not found, got:\n%s", content)
	}
}

func TestSetEntry_QuotesSpecialChars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if err := SetEntry(path, "TOKEN", "value with spaces"); err != nil {
		t.Fatalf("SetEntry: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !strings.Contains(string(data), `TOKEN="value with spaces"`) {
		t.Errorf("expected quoted value, got:\n%s", data)
	}
}

func TestSetEntry_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if err := SetEntry(path, "KEY", "val"); err != nil {
		t.Fatalf("SetEntry: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
	}
}
