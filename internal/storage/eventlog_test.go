package storage

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/events"
)

func TestEventLogger_WriteAndReadBack(t *testing.T) {
	dir := t.TempDir()
	bus := events.NewBus(64)
	defer bus.Close()

	el := NewEventLogger(dir, bus)
	defer el.Close()

	bus.Publish(events.Event{
		ID:        "evt-1",
		Type:      events.EventUserMessage,
		Timestamp: time.Now(),
		Source:    events.SourceWS,
		Payload:   map[string]any{"content": "hello"},
	})

	// Give the async subscriber time to process.
	time.Sleep(100 * time.Millisecond)

	data, err := os.ReadFile(filepath.Join(dir, "_global.jsonl"))
	if err != nil {
		t.Fatalf("read JSONL: %v", err)
	}

	var got events.Event
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != "evt-1" {
		t.Errorf("got ID %q, want %q", got.ID, "evt-1")
	}
	if got.Type != events.EventUserMessage {
		t.Errorf("got type %q, want %q", got.Type, events.EventUserMessage)
	}
}

func TestEventLogger_SessionRouting(t *testing.T) {
	dir := t.TempDir()
	bus := events.NewBus(64)
	defer bus.Close()

	el := NewEventLogger(dir, bus)
	defer el.Close()

	bus.Publish(events.Event{
		ID:        "evt-global",
		Type:      events.EventUserMessage,
		Timestamp: time.Now(),
		Source:    events.SourceWS,
	})
	bus.Publish(events.Event{
		ID:        "evt-sess",
		SessionID: "sess_abc123",
		Type:      events.EventAssistantMessage,
		Timestamp: time.Now(),
		Source:    events.SourceAgent,
	})

	time.Sleep(100 * time.Millisecond)

	// Global file should exist with global event.
	if _, err := os.Stat(filepath.Join(dir, "_global.jsonl")); err != nil {
		t.Fatalf("_global.jsonl missing: %v", err)
	}

	// Session file should exist.
	sessPath := filepath.Join(dir, "sess_abc123.jsonl")
	data, err := os.ReadFile(sessPath)
	if err != nil {
		t.Fatalf("session file missing: %v", err)
	}
	var got events.Event
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != "evt-sess" {
		t.Errorf("got ID %q, want %q", got.ID, "evt-sess")
	}
}

func TestEventLogger_StreamFiltering(t *testing.T) {
	dir := t.TempDir()
	bus := events.NewBus(64)
	defer bus.Close()

	el := NewEventLogger(dir, bus)
	defer el.Close()

	bus.Publish(events.Event{
		ID:        "evt-stream",
		Type:      events.EventAssistantStream,
		Timestamp: time.Now(),
		Source:    events.SourceAgent,
	})

	time.Sleep(100 * time.Millisecond)

	// No file should be created for stream-only events.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected no files, got %d", len(entries))
	}
}

func TestEventLogger_NonStreamEventsPersisted(t *testing.T) {
	dir := t.TempDir()
	bus := events.NewBus(64)
	defer bus.Close()

	el := NewEventLogger(dir, bus)
	defer el.Close()

	types := []events.EventType{
		events.EventUserMessage,
		events.EventAssistantMessage,
		events.EventToolCall,
	}

	for i, et := range types {
		bus.Publish(events.Event{
			ID:        string(rune('a' + i)),
			Type:      et,
			Timestamp: time.Now(),
			Source:    events.SourceAgent,
		})
	}

	time.Sleep(100 * time.Millisecond)

	f, err := os.Open(filepath.Join(dir, "_global.jsonl"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	var count int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e events.Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal line %d: %v", count, err)
		}
		count++
	}
	if count != len(types) {
		t.Errorf("got %d events, want %d", count, len(types))
	}
}

func TestEventLogger_DirectoryAutoCreation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "logs")
	bus := events.NewBus(64)
	defer bus.Close()

	el := NewEventLogger(dir, bus)
	defer el.Close()

	bus.Publish(events.Event{
		ID:        "evt-auto",
		Type:      events.EventUserMessage,
		Timestamp: time.Now(),
		Source:    events.SourceWS,
	})

	time.Sleep(100 * time.Millisecond)

	if _, err := os.Stat(filepath.Join(dir, "_global.jsonl")); err != nil {
		t.Fatalf("directory not auto-created: %v", err)
	}
}
