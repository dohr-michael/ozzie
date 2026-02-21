package events

import (
	"sync"
	"testing"
	"time"
)

func TestBusPublishSubscribe(t *testing.T) {
	bus := NewBus(64)
	defer bus.Close()

	var mu sync.Mutex
	var received []Event

	bus.Subscribe(func(e Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	}, EventUserMessage)

	bus.Publish(NewTypedEvent("test", UserMessagePayload{Content: "hello"}))
	bus.Publish(NewTypedEvent("test", AssistantStreamPayload{Phase: StreamPhaseStart}))

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != EventUserMessage {
		t.Errorf("expected user.message, got %s", received[0].Type)
	}
}

func TestBusSubscribeAll(t *testing.T) {
	bus := NewBus(64)
	defer bus.Close()

	var mu sync.Mutex
	count := 0

	bus.Subscribe(func(e Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	bus.Publish(NewTypedEvent("test", UserMessagePayload{Content: "hello"}))
	bus.Publish(NewTypedEvent("test", AssistantStreamPayload{Phase: StreamPhaseStart}))

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 events, got %d", count)
	}
}

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer(3)

	for i := 0; i < 5; i++ {
		rb.Add(NewEvent(EventUserMessage, "test", map[string]any{"i": i}))
	}

	events := rb.Get(10)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
}

func TestSubscribeChan(t *testing.T) {
	bus := NewBus(64)
	defer bus.Close()

	ch, unsub := bus.SubscribeChan(8, EventUserMessage)
	defer unsub()

	bus.Publish(NewTypedEvent("test", UserMessagePayload{Content: "hello"}))

	select {
	case e := <-ch:
		if e.Type != EventUserMessage {
			t.Errorf("expected user.message, got %s", e.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}
