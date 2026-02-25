package memory

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

// mockEmbedder is a deterministic embedder for tests (no API calls).
// It assigns each text a unique 8-dim vector based on its hash.
type mockEmbedder struct{}

func (m *mockEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	results := make([][]float64, len(texts))
	for i, text := range texts {
		results[i] = deterministicVector(text)
	}
	return results, nil
}

// deterministicVector creates a normalized 8-dim vector from text.
func deterministicVector(text string) []float64 {
	vec := make([]float64, 8)
	for i, c := range text {
		vec[i%8] += float64(c)
	}
	// Normalize
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

// --- Test helpers ---

func newTestMemoryEntry(id, title string, tags []string) *MemoryEntry {
	return &MemoryEntry{
		ID:         id,
		Title:      title,
		Type:       MemoryFact,
		Source:     "test",
		Tags:       tags,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		Confidence: 0.8,
	}
}

// memoryStoreStub is a simple in-memory Store for testing.
type memoryStoreStub struct {
	entries  []*MemoryEntry
	contents map[string]string
}

func newMemoryStoreStub() *memoryStoreStub {
	return &memoryStoreStub{contents: make(map[string]string)}
}

func (s *memoryStoreStub) Create(entry *MemoryEntry, content string) error {
	s.entries = append(s.entries, entry)
	s.contents[entry.ID] = content
	return nil
}

func (s *memoryStoreStub) Get(id string) (*MemoryEntry, string, error) {
	for _, e := range s.entries {
		if e.ID == id {
			return e, s.contents[id], nil
		}
	}
	return nil, "", nil
}

func (s *memoryStoreStub) Update(entry *MemoryEntry, content string) error {
	for i, e := range s.entries {
		if e.ID == entry.ID {
			s.entries[i] = entry
			s.contents[entry.ID] = content
			return nil
		}
	}
	return nil
}

func (s *memoryStoreStub) Delete(id string) error {
	for i, e := range s.entries {
		if e.ID == id {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			delete(s.contents, id)
			return nil
		}
	}
	return nil
}

func (s *memoryStoreStub) List() ([]*MemoryEntry, error) {
	result := make([]*MemoryEntry, len(s.entries))
	copy(result, s.entries)
	return result, nil
}

// --- Tests ---

func TestBridgeEmbedder(t *testing.T) {
	mock := &mockEmbedder{}
	ctx := context.Background()
	bridge := bridgeEmbedder(ctx, mock)

	result, err := bridge(ctx, "hello world")
	if err != nil {
		t.Fatalf("bridgeEmbedder failed: %v", err)
	}
	if len(result) != 8 {
		t.Fatalf("expected 8 dims, got %d", len(result))
	}

	// Verify float64→float32 conversion
	expected := deterministicVector("hello world")
	for i, v := range result {
		if math.Abs(float64(v)-expected[i]) > 1e-5 {
			t.Errorf("dim %d: expected %f, got %f", i, expected[i], v)
		}
	}
}

func TestVectorStore_UpsertAndQuery(t *testing.T) {
	ctx := context.Background()
	mock := &mockEmbedder{}
	dir := t.TempDir()

	vs, err := NewVectorStore(ctx, dir, mock)
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}

	// Insert 3 documents
	docs := []struct {
		id, content string
	}{
		{"doc1", "Go programming language concurrency"},
		{"doc2", "Python machine learning frameworks"},
		{"doc3", "Go goroutines and channels for concurrent programming"},
	}
	for _, d := range docs {
		if err := vs.Upsert(ctx, d.id, d.content, map[string]string{"type": "test"}); err != nil {
			t.Fatalf("Upsert %s: %v", d.id, err)
		}
	}

	if vs.Count() != 3 {
		t.Fatalf("expected count=3, got %d", vs.Count())
	}

	// Query for Go-related content
	results, err := vs.Query(ctx, "Go concurrent programming", 3)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// The most similar result should be one of the Go docs
	topID := results[0].ID
	if topID != "doc1" && topID != "doc3" {
		t.Errorf("expected top result to be doc1 or doc3, got %s", topID)
	}
}

func TestVectorStore_Delete(t *testing.T) {
	ctx := context.Background()
	mock := &mockEmbedder{}
	dir := t.TempDir()

	vs, err := NewVectorStore(ctx, dir, mock)
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}

	if err := vs.Upsert(ctx, "doc1", "test content", nil); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if vs.Count() != 1 {
		t.Fatalf("expected count=1, got %d", vs.Count())
	}

	if err := vs.Delete(ctx, "doc1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if vs.Count() != 0 {
		t.Fatalf("expected count=0 after delete, got %d", vs.Count())
	}
}

func TestHybridRetriever_KeywordOnly(t *testing.T) {
	store := newMemoryStoreStub()
	entry := newTestMemoryEntry("mem1", "Go programming", []string{"go", "programming"})
	_ = store.Create(entry, "Go is a great language for concurrency")

	// vector=nil → pure keyword fallback
	hr := NewHybridRetriever(store, nil)
	results, err := hr.Retrieve("go programming", nil, 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result from keyword fallback")
	}
	if results[0].Entry.ID != "mem1" {
		t.Errorf("expected mem1, got %s", results[0].Entry.ID)
	}
}

func TestHybridRetriever_HybridMerge(t *testing.T) {
	ctx := context.Background()
	mock := &mockEmbedder{}
	dir := t.TempDir()

	vs, err := NewVectorStore(ctx, dir, mock)
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}

	store := newMemoryStoreStub()

	// Create entries in both stores
	entries := []struct {
		id, title, content string
		tags               []string
	}{
		{"mem1", "Go concurrency", "goroutines and channels in Go", []string{"go"}},
		{"mem2", "Python ML", "machine learning with Python", []string{"python", "ml"}},
		{"mem3", "Go testing", "testing Go applications", []string{"go", "testing"}},
	}
	for _, e := range entries {
		entry := newTestMemoryEntry(e.id, e.title, e.tags)
		_ = store.Create(entry, e.content)
		text := BuildEmbedText(entry, e.content)
		_ = vs.Upsert(ctx, e.id, text, BuildEmbedMeta(entry))
	}

	hr := NewHybridRetriever(store, vs)
	results, err := hr.Retrieve("Go programming", nil, 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 hybrid result")
	}

	// All Go-related entries should appear
	ids := make(map[string]bool)
	for _, r := range results {
		ids[r.Entry.ID] = true
	}
	if !ids["mem1"] {
		t.Error("expected mem1 (Go concurrency) in results")
	}
}

func TestPipeline_EnqueueProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mock := &mockEmbedder{}
	dir := t.TempDir()

	vs, err := NewVectorStore(ctx, dir, mock)
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}

	p := NewPipeline(vs, 10)
	p.Start(ctx)

	// Enqueue a job
	p.Enqueue(EmbedJob{
		ID:      "doc1",
		Content: "test content for embedding",
		Meta:    map[string]string{"type": "test"},
	})

	// Stop waits for processing to complete
	p.Stop()

	if vs.Count() != 1 {
		t.Fatalf("expected 1 document after pipeline processing, got %d", vs.Count())
	}
}

func TestPipeline_Delete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mock := &mockEmbedder{}
	dir := t.TempDir()

	vs, err := NewVectorStore(ctx, dir, mock)
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}

	// Pre-populate
	if err := vs.Upsert(ctx, "doc1", "some content", nil); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	p := NewPipeline(vs, 10)
	p.Start(ctx)

	p.Enqueue(EmbedJob{ID: "doc1", Delete: true})
	p.Stop()

	if vs.Count() != 0 {
		t.Fatalf("expected 0 documents after delete, got %d", vs.Count())
	}
}

func TestReindex(t *testing.T) {
	ctx := context.Background()
	mock := &mockEmbedder{}
	dir := t.TempDir()

	vs, err := NewVectorStore(ctx, dir, mock)
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}

	store := newMemoryStoreStub()
	for i := range 5 {
		id := "mem" + string(rune('1'+i))
		entry := newTestMemoryEntry(id, "Memory "+id, []string{"test"})
		_ = store.Create(entry, "Content for "+id)
	}

	stats, err := Reindex(ctx, store, vs)
	if err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	if stats.Total != 5 {
		t.Errorf("expected total=5, got %d", stats.Total)
	}
	if stats.Indexed != 5 {
		t.Errorf("expected indexed=5, got %d", stats.Indexed)
	}
	if stats.Errors != 0 {
		t.Errorf("expected errors=0, got %d", stats.Errors)
	}
	if vs.Count() != 5 {
		t.Errorf("expected vector count=5, got %d", vs.Count())
	}
}

func TestBuildEmbedText(t *testing.T) {
	entry := newTestMemoryEntry("mem1", "Go Programming", []string{"go", "dev"})
	text := BuildEmbedText(entry, "Go is awesome")
	expected := "Go Programming [go, dev]\nGo is awesome"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestBuildEmbedText_NoTags(t *testing.T) {
	entry := newTestMemoryEntry("mem1", "Simple Memory", nil)
	text := BuildEmbedText(entry, "Some content")
	expected := "Simple Memory\nSome content"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}
