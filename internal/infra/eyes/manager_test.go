package eyes

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/dohr-michael/ozzie/internal/core/policy"
	"github.com/dohr-michael/ozzie/internal/infra/sessions"
	"github.com/dohr-michael/ozzie/pkg/connector"
)

// fakeConnector implements connector.Connector for testing.
type fakeConnector struct {
	name    string
	handler connector.IncomingHandler

	mu       sync.Mutex
	sent     []connector.OutgoingMessage
	typings  int
	startErr error
}

func (f *fakeConnector) Name() string { return f.name }

func (f *fakeConnector) Start(ctx context.Context, handler connector.IncomingHandler) error {
	f.handler = handler
	if f.startErr != nil {
		return f.startErr
	}
	<-ctx.Done()
	return nil
}

func (f *fakeConnector) Send(_ context.Context, msg connector.OutgoingMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, msg)
	return nil
}

func (f *fakeConnector) SendTyping(_ context.Context, _ string) func() {
	f.mu.Lock()
	f.typings++
	f.mu.Unlock()
	return func() {}
}

func (f *fakeConnector) Stop() error { return nil }

func (f *fakeConnector) Sent() []connector.OutgoingMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]connector.OutgoingMessage, len(f.sent))
	copy(out, f.sent)
	return out
}

func (f *fakeConnector) Typings() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.typings
}

// simulateIncoming triggers the handler as if a message was received.
func (f *fakeConnector) simulateIncoming(msg connector.IncomingMessage) {
	if f.handler != nil {
		f.handler(context.Background(), msg)
	}
}

func TestManagerIncomingPaired(t *testing.T) {
	bus := events.NewBus(128)
	defer bus.Close()

	dir := t.TempDir()
	sessStore := sessions.NewFileStore(dir)
	pairStore := policy.NewPairingStore(dir)
	sessionMap := NewSessionMap(dir)

	// Add a pairing
	_ = pairStore.Add(policy.Pairing{
		Key:        policy.PairingKey{Platform: "test", ServerID: "s1", ChannelID: "c1", UserID: "u1"},
		PolicyName: "support",
	})

	fc := &fakeConnector{name: "test"}

	// Collect user message events — subscribe BEFORE manager start
	var received []events.Event
	var mu sync.Mutex
	unsub := bus.Subscribe(func(e events.Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	}, events.EventUserMessage)
	defer unsub()

	mgr := NewManager(ManagerConfig{
		Bus:          bus,
		SessionStore: sessStore,
		PairingStore: pairStore,
		SessionMap:   sessionMap,
		Connectors:   []connector.Connector{fc},
	})
	mgr.Start()
	defer mgr.Stop()

	// Wait for connector to start
	time.Sleep(50 * time.Millisecond)

	// Simulate incoming message
	fc.simulateIncoming(connector.IncomingMessage{
		Identity: connector.Identity{
			Platform:  "test",
			UserID:    "u1",
			Name:      "TestUser",
			ServerID:  "s1",
			ChannelID: "c1",
		},
		Content:   "hello",
		ChannelID: "c1",
		MessageID: "m1",
	})

	// Wait for event propagation
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatal("expected user message event")
	}
	if received[0].SessionID == "" {
		t.Fatal("expected session ID on event")
	}

	// Verify session was mapped
	sid, ok := sessionMap.Get("test", "s1", "c1", "u1")
	if !ok {
		t.Fatal("expected session mapping")
	}
	if sid != received[0].SessionID {
		t.Fatalf("session mismatch: map=%q event=%q", sid, received[0].SessionID)
	}

	// Verify typing was started
	if fc.Typings() == 0 {
		t.Fatal("expected typing indicator")
	}
}

func TestManagerUnpairedUser(t *testing.T) {
	bus := events.NewBus(128)
	defer bus.Close()

	dir := t.TempDir()
	sessStore := sessions.NewFileStore(dir)
	pairStore := policy.NewPairingStore(dir) // empty — no pairings
	sessionMap := NewSessionMap(dir)

	fc := &fakeConnector{name: "test"}

	mgr := NewManager(ManagerConfig{
		Bus:          bus,
		SessionStore: sessStore,
		PairingStore: pairStore,
		SessionMap:   sessionMap,
		Connectors:   []connector.Connector{fc},
	})
	mgr.Start()
	defer mgr.Stop()

	time.Sleep(50 * time.Millisecond)

	// Collect pairing request events
	var pairingReqs []events.Event
	var mu sync.Mutex
	unsub := bus.Subscribe(func(e events.Event) {
		mu.Lock()
		pairingReqs = append(pairingReqs, e)
		mu.Unlock()
	}, events.EventPairingRequest)
	defer unsub()

	fc.simulateIncoming(connector.IncomingMessage{
		Identity: connector.Identity{
			Platform:  "test",
			UserID:    "unknown",
			Name:      "Unknown",
			ServerID:  "s1",
			ChannelID: "c1",
		},
		Content:   "hello",
		ChannelID: "c1",
		MessageID: "m1",
	})

	time.Sleep(100 * time.Millisecond)

	// Should have sent a pending reply
	sent := fc.Sent()
	if len(sent) == 0 {
		t.Fatal("expected pending reply")
	}
	if sent[0].Content != "Your access is pending approval. An administrator has been notified." {
		t.Fatalf("unexpected reply: %q", sent[0].Content)
	}

	// Should have published pairing request
	mu.Lock()
	defer mu.Unlock()
	if len(pairingReqs) == 0 {
		t.Fatal("expected pairing request event")
	}
}

func TestManagerResponseRouting(t *testing.T) {
	bus := events.NewBus(128)
	defer bus.Close()

	dir := t.TempDir()
	sessStore := sessions.NewFileStore(dir)
	pairStore := policy.NewPairingStore(dir)
	sessionMap := NewSessionMap(dir)

	_ = pairStore.Add(policy.Pairing{
		Key:        policy.PairingKey{Platform: "test", ServerID: "s1", ChannelID: "c1", UserID: "u1"},
		PolicyName: "support",
	})

	fc := &fakeConnector{name: "test"}

	mgr := NewManager(ManagerConfig{
		Bus:          bus,
		SessionStore: sessStore,
		PairingStore: pairStore,
		SessionMap:   sessionMap,
		Connectors:   []connector.Connector{fc},
	})
	mgr.Start()
	defer mgr.Stop()

	time.Sleep(50 * time.Millisecond)

	// Simulate incoming to create a session
	fc.simulateIncoming(connector.IncomingMessage{
		Identity: connector.Identity{
			Platform:  "test",
			UserID:    "u1",
			Name:      "User1",
			ServerID:  "s1",
			ChannelID: "c1",
		},
		Content:   "hello",
		ChannelID: "c1",
		MessageID: "m1",
	})

	time.Sleep(100 * time.Millisecond)

	// Get the session ID
	sid, ok := sessionMap.Get("test", "s1", "c1", "u1")
	if !ok {
		t.Fatal("expected session")
	}

	// Simulate assistant response
	bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.AssistantMessagePayload{
		Content: "Hello back!",
	}, sid))

	time.Sleep(100 * time.Millisecond)

	// Check connector received the message
	sent := fc.Sent()
	if len(sent) == 0 {
		t.Fatal("expected outgoing message")
	}
	if sent[0].Content != "Hello back!" {
		t.Fatalf("unexpected content: %q", sent[0].Content)
	}
	if sent[0].ChannelID != "c1" {
		t.Fatalf("unexpected channel: %q", sent[0].ChannelID)
	}
}
