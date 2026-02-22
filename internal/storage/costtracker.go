package storage

import (
	"log/slog"
	"sync"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/sessions"
)

// CostTracker subscribes to LLM call events and accumulates token usage per session.
type CostTracker struct {
	mu          sync.Mutex
	bus         *events.Bus
	store       sessions.Store
	unsubscribe func()
}

// NewCostTracker creates a CostTracker that listens for LLM response events.
func NewCostTracker(bus *events.Bus, store sessions.Store) *CostTracker {
	ct := &CostTracker{
		bus:   bus,
		store: store,
	}
	ct.unsubscribe = bus.Subscribe(ct.handleEvent, events.EventLLMCall)
	return ct
}

// Close unsubscribes the tracker from the event bus.
func (ct *CostTracker) Close() {
	if ct.unsubscribe != nil {
		ct.unsubscribe()
	}
}

func (ct *CostTracker) handleEvent(e events.Event) {
	if e.SessionID == "" {
		return
	}

	payload, ok := events.GetLLMCallPayload(e)
	if !ok {
		return
	}

	if payload.Phase != "response" {
		return
	}

	if payload.TokensInput == 0 && payload.TokensOutput == 0 {
		return
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	sess, err := ct.store.Get(e.SessionID)
	if err != nil {
		slog.Debug("cost tracker: session not found", "session_id", e.SessionID, "error", err)
		return
	}

	sess.TokenUsage.Input += payload.TokensInput
	sess.TokenUsage.Output += payload.TokensOutput

	if err := ct.store.UpdateMeta(sess); err != nil {
		slog.Error("cost tracker: update meta", "session_id", e.SessionID, "error", err)
	}
}
