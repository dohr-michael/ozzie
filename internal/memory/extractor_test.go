package memory

import (
	"context"
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/events"
)

func TestParseLessons_ValidJSON(t *testing.T) {
	input := `[{"title":"Use go test -race","content":"Always run race detector for concurrent code","tags":["go","testing"]}]`
	lessons := parseLessons(input)
	if len(lessons) != 1 {
		t.Fatalf("expected 1 lesson, got %d", len(lessons))
	}
	if lessons[0].Title != "Use go test -race" {
		t.Errorf("unexpected title: %s", lessons[0].Title)
	}
	if len(lessons[0].Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(lessons[0].Tags))
	}
}

func TestParseLessons_EmptyArray(t *testing.T) {
	lessons := parseLessons("[]")
	if len(lessons) != 0 {
		t.Errorf("expected 0 lessons, got %d", len(lessons))
	}
}

func TestParseLessons_MarkdownFences(t *testing.T) {
	input := "```json\n" + `[{"title":"Test","content":"Content","tags":["a"]}]` + "\n```"
	lessons := parseLessons(input)
	if len(lessons) != 1 {
		t.Fatalf("expected 1 lesson, got %d", len(lessons))
	}
	if lessons[0].Title != "Test" {
		t.Errorf("unexpected title: %s", lessons[0].Title)
	}
}

func TestParseLessons_CapsAtThree(t *testing.T) {
	input := `[
		{"title":"A","content":"a","tags":[]},
		{"title":"B","content":"b","tags":[]},
		{"title":"C","content":"c","tags":[]},
		{"title":"D","content":"d","tags":[]},
		{"title":"E","content":"e","tags":[]}
	]`
	lessons := parseLessons(input)
	if len(lessons) != 3 {
		t.Errorf("expected max 3 lessons, got %d", len(lessons))
	}
}

func TestParseLessons_InvalidJSON(t *testing.T) {
	lessons := parseLessons("not valid json")
	if len(lessons) != 0 {
		t.Errorf("expected 0 lessons for invalid JSON, got %d", len(lessons))
	}
}

func TestParseLessons_SkipsEmptyEntries(t *testing.T) {
	input := `[{"title":"","content":"","tags":[]},{"title":"Valid","content":"c","tags":[]}]`
	lessons := parseLessons(input)
	if len(lessons) != 1 {
		t.Fatalf("expected 1 valid lesson, got %d", len(lessons))
	}
	if lessons[0].Title != "Valid" {
		t.Errorf("unexpected title: %s", lessons[0].Title)
	}
}

// --- Integration test with mocks ---

type mockTaskReader struct {
	outputs map[string]string
}

func (m *mockTaskReader) ReadOutput(taskID string) (string, error) {
	return m.outputs[taskID], nil
}

type mockSummarizer struct {
	response string
}

func (m *mockSummarizer) Summarize(_ context.Context, _ string) (string, error) {
	return m.response, nil
}

func TestExtractor_ExtractsAndStoresLessons(t *testing.T) {
	store := newMemoryStoreStub()
	bus := events.NewBus(16)
	defer bus.Close()

	taskReader := &mockTaskReader{
		outputs: map[string]string{
			"task-123": "Implemented user auth with JWT tokens. Discovered that bcrypt cost=12 is optimal for our use case.",
		},
	}

	summarizer := &mockSummarizer{
		response: `[{"title":"JWT auth pattern","content":"Use bcrypt cost=12 for password hashing","tags":["auth","security"]}]`,
	}

	extractor := NewExtractor(ExtractorConfig{
		Store:      store,
		Pipeline:   nil, // no embedding pipeline for this test
		TaskReader: taskReader,
		Summarizer: summarizer,
		Bus:        bus,
	})
	extractor.Start()
	defer extractor.Stop()

	// Emit task.completed event
	bus.Publish(events.NewTypedEvent(
		events.SourceTask,
		events.TaskCompletedPayload{
			TaskID: "task-123",
			Title:  "Implement user authentication",
		},
	))

	// Wait for async extraction
	time.Sleep(300 * time.Millisecond)

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 stored memory, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Title != "JWT auth pattern" {
		t.Errorf("unexpected title: %s", entry.Title)
	}
	if entry.Type != MemoryProcedure {
		t.Errorf("expected type=procedure, got %s", entry.Type)
	}
	if entry.Source != "task:task-123" {
		t.Errorf("unexpected source: %s", entry.Source)
	}
	content := store.contents[entry.ID]
	if content != "Use bcrypt cost=12 for password hashing" {
		t.Errorf("unexpected content: %s", content)
	}
}
