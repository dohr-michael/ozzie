package storage

import (
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/sessions"
)

func publishLLMEvent(bus *events.Bus, sessionID, phase string, tokensIn, tokensOut int) {
	payload := events.LLMCallPayload{
		Phase:        phase,
		Model:        "test-model",
		TokensInput:  tokensIn,
		TokensOutput: tokensOut,
	}
	bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, payload, sessionID))
}

func TestCostTracker_Accumulation(t *testing.T) {
	bus := events.NewBus(64)
	defer bus.Close()

	dir := t.TempDir()
	store := sessions.NewFileStore(dir)

	sess, err := store.Create()
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ct := NewCostTracker(bus, store)
	defer ct.Close()

	publishLLMEvent(bus, sess.ID, "response", 100, 50)
	publishLLMEvent(bus, sess.ID, "response", 200, 80)

	time.Sleep(150 * time.Millisecond)

	got, err := store.Get(sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}

	if got.TokenUsage.Input != 300 {
		t.Errorf("input tokens: got %d, want 300", got.TokenUsage.Input)
	}
	if got.TokenUsage.Output != 130 {
		t.Errorf("output tokens: got %d, want 130", got.TokenUsage.Output)
	}
}

func TestCostTracker_PhaseFiltering(t *testing.T) {
	bus := events.NewBus(64)
	defer bus.Close()

	dir := t.TempDir()
	store := sessions.NewFileStore(dir)

	sess, err := store.Create()
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ct := NewCostTracker(bus, store)
	defer ct.Close()

	publishLLMEvent(bus, sess.ID, "request", 100, 0)
	publishLLMEvent(bus, sess.ID, "error", 0, 0)

	time.Sleep(150 * time.Millisecond)

	got, err := store.Get(sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}

	if got.TokenUsage.Input != 0 {
		t.Errorf("input tokens: got %d, want 0", got.TokenUsage.Input)
	}
	if got.TokenUsage.Output != 0 {
		t.Errorf("output tokens: got %d, want 0", got.TokenUsage.Output)
	}
}

func TestCostTracker_NoSessionID(t *testing.T) {
	bus := events.NewBus(64)
	defer bus.Close()

	dir := t.TempDir()
	store := sessions.NewFileStore(dir)

	ct := NewCostTracker(bus, store)
	defer ct.Close()

	// Publish without session ID — should not panic.
	publishLLMEvent(bus, "", "response", 100, 50)

	time.Sleep(150 * time.Millisecond)
}

func TestCostTracker_ZeroTokens(t *testing.T) {
	bus := events.NewBus(64)
	defer bus.Close()

	dir := t.TempDir()
	store := sessions.NewFileStore(dir)

	sess, err := store.Create()
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ct := NewCostTracker(bus, store)
	defer ct.Close()

	publishLLMEvent(bus, sess.ID, "response", 0, 0)

	time.Sleep(150 * time.Millisecond)

	got, err := store.Get(sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}

	if got.TokenUsage.Input != 0 {
		t.Errorf("input tokens: got %d, want 0", got.TokenUsage.Input)
	}
	if got.TokenUsage.Output != 0 {
		t.Errorf("output tokens: got %d, want 0", got.TokenUsage.Output)
	}
}
