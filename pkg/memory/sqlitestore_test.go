package memory

import (
	"os"
	"testing"
	"time"
)

func TestSQLiteStore_CRUD(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	// Create
	entry := &MemoryEntry{
		Title:      "Go concurrency",
		Type:       MemoryFact,
		Source:     "test",
		Tags:       []string{"go", "concurrency"},
		Importance: ImportanceNormal,
	}
	if err := store.Create(entry, "Goroutines and channels"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("expected ID to be set")
	}

	// Get
	got, content, err := store.Get(entry.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Go concurrency" {
		t.Errorf("expected title 'Go concurrency', got %q", got.Title)
	}
	if content != "Goroutines and channels" {
		t.Errorf("expected content 'Goroutines and channels', got %q", content)
	}
	if got.Importance != ImportanceNormal {
		t.Errorf("expected importance 'normal', got %q", got.Importance)
	}
	if len(got.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(got.Tags))
	}

	// Update
	got.Title = "Go concurrency patterns"
	got.Confidence = 0.9
	if err := store.Update(got, "Goroutines, channels, and select"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, updatedContent, _ := store.Get(entry.ID)
	if updated.Title != "Go concurrency patterns" {
		t.Errorf("expected updated title, got %q", updated.Title)
	}
	if updatedContent != "Goroutines, channels, and select" {
		t.Errorf("expected updated content, got %q", updatedContent)
	}

	// List
	entries, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Delete
	if err := store.Delete(entry.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	entries, _ = store.List()
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after delete, got %d", len(entries))
	}
}

func TestSQLiteStore_FTS5(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	// Insert test data
	entries := []struct {
		title, content string
		tags           []string
	}{
		{"Go concurrency", "Goroutines and channels enable concurrent programming", []string{"go"}},
		{"Python ML", "Machine learning with scikit-learn and TensorFlow", []string{"python", "ml"}},
		{"Go testing", "Table-driven tests and benchmarks in Go", []string{"go", "testing"}},
	}
	for _, e := range entries {
		_ = store.Create(&MemoryEntry{
			Title:  e.title,
			Type:   MemoryFact,
			Source: "test",
			Tags:   e.tags,
		}, e.content)
	}

	// Search for Go
	results, err := store.SearchFTS("go concurrency", 10)
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected FTS results for 'go concurrency'")
	}

	// First result should be Go-related
	found := false
	for _, r := range results {
		if r.Title == "Go concurrency" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Go concurrency' in FTS results")
	}
}

func TestSQLiteStore_MergedExcludedFromList(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	// Create target entry
	target := &MemoryEntry{Title: "Target", Type: MemoryFact, Source: "test"}
	_ = store.Create(target, "target content")

	// Create source entry, then merge it
	source := &MemoryEntry{Title: "Source", Type: MemoryFact, Source: "test"}
	_ = store.Create(source, "source content")

	source.MergedInto = target.ID
	_ = store.Update(source, "source content")

	// List should exclude merged entries
	entries, _ := store.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 active entry, got %d", len(entries))
	}
	if entries[0].ID != target.ID {
		t.Errorf("expected target entry, got %s", entries[0].ID)
	}
}

func TestSQLiteStore_Touch(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	entry := &MemoryEntry{Title: "Test", Type: MemoryFact, Source: "test"}
	_ = store.Create(entry, "content")

	now := time.Now().Add(time.Hour)
	if err := store.Touch(entry.ID, now); err != nil {
		t.Fatalf("Touch: %v", err)
	}

	got, _, _ := store.Get(entry.ID)
	if got.LastUsedAt.Before(now.Add(-time.Second)) {
		t.Errorf("expected LastUsedAt updated to ~%v, got %v", now, got.LastUsedAt)
	}
}

func TestSQLiteStore_MarkdownSync(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	entry := &MemoryEntry{Title: "Test MD", Type: MemoryFact, Source: "test"}
	_ = store.Create(entry, "# Hello\nWorld")

	// Check .md file exists
	mdPath := store.contentPath(entry.ID)
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("expected .md file at %s, got error: %v", mdPath, err)
	}

	// Delete should remove .md
	_ = store.Delete(entry.ID)
	if _, err := os.Stat(mdPath); !os.IsNotExist(err) {
		t.Error("expected .md file to be removed after delete")
	}
}

func TestSQLiteStore_Importance(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	for _, imp := range ValidImportanceLevels {
		entry := &MemoryEntry{
			Title:      "Test " + string(imp),
			Type:       MemoryFact,
			Source:     "test",
			Importance: imp,
		}
		if err := store.Create(entry, "content"); err != nil {
			t.Fatalf("Create with importance %q: %v", imp, err)
		}
		got, _, _ := store.Get(entry.ID)
		if got.Importance != imp {
			t.Errorf("expected importance %q, got %q", imp, got.Importance)
		}
	}
}
