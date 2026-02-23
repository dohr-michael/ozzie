package dirstore

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

type testMeta struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestWriteReadMeta(t *testing.T) {
	ds := NewDirStore(t.TempDir(), "thing")
	id := "abc123"

	if err := ds.EnsureDir(id); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	want := testMeta{Name: "hello", Value: 42}
	if err := ds.WriteMeta(id, want); err != nil {
		t.Fatalf("WriteMeta: %v", err)
	}

	var got testMeta
	if err := ds.ReadMeta(id, &got); err != nil {
		t.Fatalf("ReadMeta: %v", err)
	}

	if got != want {
		t.Errorf("ReadMeta = %+v, want %+v", got, want)
	}
}

func TestReadMetaNotFound(t *testing.T) {
	ds := NewDirStore(t.TempDir(), "widget")

	var out testMeta
	err := ds.ReadMeta("nonexistent", &out)
	if err == nil {
		t.Fatal("expected error for missing meta")
	}
	if want := "widget not found: nonexistent"; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestListDirs(t *testing.T) {
	base := t.TempDir()
	ds := NewDirStore(base, "item")

	// Create some directories and a file (should be ignored)
	for _, name := range []string{"dir_a", "dir_b", "dir_c"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "not_a_dir.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	dirs, err := ds.ListDirs()
	if err != nil {
		t.Fatalf("ListDirs: %v", err)
	}

	sort.Strings(dirs)
	want := []string{"dir_a", "dir_b", "dir_c"}
	if len(dirs) != len(want) {
		t.Fatalf("ListDirs = %v, want %v", dirs, want)
	}
	for i, d := range dirs {
		if d != want[i] {
			t.Errorf("dirs[%d] = %q, want %q", i, d, want[i])
		}
	}
}

func TestListDirsNonExistent(t *testing.T) {
	ds := NewDirStore(filepath.Join(t.TempDir(), "nope"), "item")

	dirs, err := ds.ListDirs()
	if err != nil {
		t.Fatalf("ListDirs: %v", err)
	}
	if dirs != nil {
		t.Errorf("expected nil, got %v", dirs)
	}
}

type testLine struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

func TestAppendAndLoadJSONL(t *testing.T) {
	ds := NewDirStore(t.TempDir(), "thing")
	id := "entity1"

	if err := ds.EnsureDir(id); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	lines := []testLine{
		{ID: 1, Text: "first"},
		{ID: 2, Text: "second"},
		{ID: 3, Text: "third"},
	}

	for _, l := range lines {
		if err := ds.AppendJSONL(id, "data.jsonl", l); err != nil {
			t.Fatalf("AppendJSONL: %v", err)
		}
	}

	got, err := LoadJSONL[testLine](ds, id, "data.jsonl")
	if err != nil {
		t.Fatalf("LoadJSONL: %v", err)
	}

	if len(got) != len(lines) {
		t.Fatalf("LoadJSONL returned %d items, want %d", len(got), len(lines))
	}
	for i, item := range got {
		if item != lines[i] {
			t.Errorf("item[%d] = %+v, want %+v", i, item, lines[i])
		}
	}
}

func TestLoadJSONLEmpty(t *testing.T) {
	ds := NewDirStore(t.TempDir(), "thing")

	got, err := LoadJSONL[testLine](ds, "nonexistent", "data.jsonl")
	if err != nil {
		t.Fatalf("LoadJSONL: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestWriteFileAtomic(t *testing.T) {
	ds := NewDirStore(t.TempDir(), "thing")
	id := "entity1"

	if err := ds.EnsureDir(id); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	content := []byte("hello world")
	if err := ds.WriteFileAtomic(id, "output.md", content); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	got, err := ds.ReadFileContent(id, "output.md")
	if err != nil {
		t.Fatalf("ReadFileContent: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("ReadFileContent = %q, want %q", got, content)
	}
}

func TestReadFileContentNotFound(t *testing.T) {
	ds := NewDirStore(t.TempDir(), "thing")

	got, err := ds.ReadFileContent("nonexistent", "output.md")
	if err != nil {
		t.Fatalf("ReadFileContent: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestEnsureDirRemoveDir(t *testing.T) {
	ds := NewDirStore(t.TempDir(), "thing")
	id := "entity1"

	if err := ds.EnsureDir(id); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(ds.Dir(id))
	if err != nil {
		t.Fatalf("Stat after EnsureDir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}

	if err := ds.RemoveDir(id); err != nil {
		t.Fatalf("RemoveDir: %v", err)
	}

	// Verify directory removed
	_, err = os.Stat(ds.Dir(id))
	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist after RemoveDir, got: %v", err)
	}
}
