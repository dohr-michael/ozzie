// Package events provides an in-memory event bus using Go channels.
package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrBusClosed = errors.New("event bus is closed")
)

// EventType represents the type of event.
type EventType string

const (
	// Incoming / Outgoing (external boundaries)
	EventIncomingMessage EventType = "incoming.message"
	EventOutgoingMessage EventType = "outgoing.message"

	// User → Agent
	EventUserMessage EventType = "user.message"

	// Agent → Client: Assistant
	EventAssistantStream  EventType = "assistant.stream"
	EventAssistantMessage EventType = "assistant.message"

	// Agent → Client: Tools
	EventToolCall             EventType = "tool.call"
	EventToolCallConfirmation EventType = "tool.call.confirmation"

	// Agent ↔ Client: Prompts
	EventPromptRequest  EventType = "prompt.request"
	EventPromptResponse EventType = "prompt.response"

	// Internal (analytics/tracing)
	EventLLMCall EventType = "internal.llm.call"

	// Session lifecycle
	EventSessionCreated EventType = "session.created"
	EventSessionClosed  EventType = "session.closed"

	// Scheduler
	EventScheduleTrigger EventType = "schedule.trigger"

	// Skills
	EventSkillStarted       EventType = "skill.started"
	EventSkillCompleted     EventType = "skill.completed"
	EventSkillStepStarted   EventType = "skill.step.started"
	EventSkillStepCompleted EventType = "skill.step.completed"
)

// EventSource identifies the component that emitted an event.
type EventSource string

const (
	SourceAgent  EventSource = "agent"
	SourceHub    EventSource = "hub"
	SourceWS     EventSource = "ws"
	SourcePlugin EventSource = "plugin"
	SourceSkill  EventSource = "skill"
)

// Event represents an event in the system.
type Event struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id,omitempty"`
	Type      EventType      `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Source    EventSource    `json:"source"`
	Payload   map[string]any `json:"payload"`
}

// eventIDCounter is used to generate sequential event IDs.
var eventIDCounter uint64

// NewEvent creates a new event with the current timestamp.
func NewEvent(eventType EventType, source EventSource, payload map[string]any) Event {
	return Event{
		ID:        generateEventID(),
		Type:      eventType,
		Timestamp: time.Now(),
		Source:    source,
		Payload:   payload,
	}
}

// NewEventWithSession creates a new event with session context.
func NewEventWithSession(eventType EventType, source EventSource, payload map[string]any, sessionID string) Event {
	return Event{
		ID:        generateEventID(),
		SessionID: sessionID,
		Type:      eventType,
		Timestamp: time.Now(),
		Source:    source,
		Payload:   payload,
	}
}

func generateEventID() string {
	seq := atomic.AddUint64(&eventIDCounter, 1)
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), seq)
}

// Subscriber is a function that receives events.
type Subscriber func(Event)

type subscription struct {
	id         int
	eventTypes []EventType
	handler    Subscriber
}

// Bus is an in-memory event bus using Go channels.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[int]*subscription
	nextID      int
	eventChan   chan Event
	bufferSize  int
	ringBuffer  *RingBuffer
	closed      bool
	done        chan struct{}
}

// NewBus creates a new event bus.
func NewBus(bufferSize int) *Bus {
	b := &Bus{
		subscribers: make(map[int]*subscription),
		eventChan:   make(chan Event, bufferSize),
		bufferSize:  bufferSize,
		ringBuffer:  NewRingBuffer(bufferSize),
		done:        make(chan struct{}),
	}
	go b.dispatch()
	return b
}

func (b *Bus) dispatch() {
	for {
		select {
		case event := <-b.eventChan:
			b.ringBuffer.Add(event)
			b.notifySubscribers(event)
		case <-b.done:
			return
		}
	}
}

func (b *Bus) notifySubscribers(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		if b.matches(sub, event) {
			go sub.handler(event)
		}
	}
}

func (b *Bus) matches(sub *subscription, event Event) bool {
	if len(sub.eventTypes) == 0 {
		return true
	}
	for _, t := range sub.eventTypes {
		if t == event.Type {
			return true
		}
	}
	return false
}

// Publish sends an event to the bus.
func (b *Bus) Publish(event Event) {
	b.mu.RLock()
	closed := b.closed
	b.mu.RUnlock()

	if closed {
		return
	}

	select {
	case b.eventChan <- event:
	default:
	}
}

// PublishAsync sends an event with context cancellation support.
func (b *Bus) PublishAsync(ctx context.Context, event Event) error {
	b.mu.RLock()
	closed := b.closed
	b.mu.RUnlock()

	if closed {
		return ErrBusClosed
	}

	select {
	case b.eventChan <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Subscribe registers a handler for specific event types.
// Returns an unsubscribe function.
func (b *Bus) Subscribe(handler Subscriber, eventTypes ...EventType) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++

	b.subscribers[id] = &subscription{
		id:         id,
		eventTypes: eventTypes,
		handler:    handler,
	}

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subscribers, id)
	}
}

// SubscribeChan returns a channel that receives events.
func (b *Bus) SubscribeChan(bufSize int, eventTypes ...EventType) (<-chan Event, func()) {
	ch := make(chan Event, bufSize)

	unsubscribe := b.Subscribe(func(e Event) {
		select {
		case ch <- e:
		default:
		}
	}, eventTypes...)

	return ch, func() {
		unsubscribe()
		close(ch)
	}
}

// History returns recent events from the ring buffer.
func (b *Bus) History(limit int) []Event {
	return b.ringBuffer.Get(limit)
}

// Close shuts down the event bus.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.closed = true
	close(b.done)
	close(b.eventChan)
}

// RingBuffer is a circular buffer for storing recent events.
type RingBuffer struct {
	mu     sync.RWMutex
	events []Event
	size   int
	pos    int
	count  int
}

// NewRingBuffer creates a new ring buffer.
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		events: make([]Event, size),
		size:   size,
	}
}

func (r *RingBuffer) Add(event Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events[r.pos] = event
	r.pos = (r.pos + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

func (r *RingBuffer) Get(n int) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if n > r.count {
		n = r.count
	}
	if n <= 0 {
		return nil
	}

	result := make([]Event, n)
	start := (r.pos - n + r.size) % r.size
	for i := 0; i < n; i++ {
		result[i] = r.events[(start+i)%r.size]
	}
	return result
}

func (r *RingBuffer) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pos = 0
	r.count = 0
}
