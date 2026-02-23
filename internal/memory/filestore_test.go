package memory

import (
	"strings"
	"testing"
)

func TestFileStore_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	entry := &MemoryEntry{
		Title:  "Tabs preference",
		Type:   MemoryPreference,
		Source: "user",
		Tags:   []string{"editor", "formatting"},
	}
	content := "The user prefers tabs over spaces."

	if err := fs.Create(entry, content); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if entry.ID == "" {
		t.Fatal("expected ID to be generated")
	}
	if !strings.HasPrefix(entry.ID, "mem_") {
		t.Fatalf("expected ID prefix 'mem_', got %q", entry.ID)
	}

	// Get
	got, gotContent, err := fs.Get(entry.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Tabs preference" {
		t.Fatalf("expected title %q, got %q", "Tabs preference", got.Title)
	}
	if gotContent != content {
		t.Fatalf("expected content %q, got %q", content, gotContent)
	}
	if got.Confidence != 0.8 {
		t.Fatalf("expected default confidence 0.8, got %f", got.Confidence)
	}
}

func TestFileStore_Update(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	entry := &MemoryEntry{
		Title:  "Old title",
		Type:   MemoryFact,
		Source: "agent",
	}
	if err := fs.Create(entry, "old content"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	entry.Title = "Updated title"
	if err := fs.Update(entry, "new content"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, gotContent, err := fs.Get(entry.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Title != "Updated title" {
		t.Fatalf("expected title %q, got %q", "Updated title", got.Title)
	}
	if gotContent != "new content" {
		t.Fatalf("expected content %q, got %q", "new content", gotContent)
	}
}

func TestFileStore_Delete(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	entry := &MemoryEntry{
		Title:  "To delete",
		Type:   MemoryFact,
		Source: "agent",
	}
	if err := fs.Create(entry, "content"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := fs.Delete(entry.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, _, err := fs.Get(entry.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestFileStore_List(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	for i := 0; i < 3; i++ {
		entry := &MemoryEntry{
			Title:  "Entry",
			Type:   MemoryFact,
			Source: "test",
		}
		if err := fs.Create(entry, "content"); err != nil {
			t.Fatalf("Create #%d: %v", i, err)
		}
	}

	list, err := fs.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(list))
	}
}

func TestFileStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	_, _, err := fs.Get("mem_nonexist")
	if err == nil {
		t.Fatal("expected error for nonexistent entry")
	}
}

func TestFileStore_DeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir)

	err := fs.Delete("mem_nonexist")
	if err == nil {
		t.Fatal("expected error for nonexistent entry")
	}
}

func TestFileStore_Persistence(t *testing.T) {
	dir := t.TempDir()

	// First instance — create
	fs1 := NewFileStore(dir)
	entry := &MemoryEntry{
		Title:  "Persistent",
		Type:   MemoryPreference,
		Source: "user",
	}
	if err := fs1.Create(entry, "persisted content"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Second instance — should reload index
	fs2 := NewFileStore(dir)
	list, err := fs2.List()
	if err != nil {
		t.Fatalf("List from new instance: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 entry in new instance, got %d", len(list))
	}
	if list[0].Title != "Persistent" {
		t.Fatalf("expected title %q, got %q", "Persistent", list[0].Title)
	}

	_, content, err := fs2.Get(entry.ID)
	if err != nil {
		t.Fatalf("Get from new instance: %v", err)
	}
	if content != "persisted content" {
		t.Fatalf("expected content %q, got %q", "persisted content", content)
	}
}
