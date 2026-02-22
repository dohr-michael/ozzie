package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/sessions"
)

// waitForEvents polls the bus history until at least n events are present.
func waitForEvents(bus *events.Bus, n int) {
	for i := 0; i < 200; i++ {
		if len(bus.History(100)) >= n {
			return
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	bus := events.NewBus(64)
	t.Cleanup(func() { bus.Close() })

	store := sessions.NewFileStore(t.TempDir())
	return NewServer(bus, store, "localhost", 0)
}

func TestHandleHealth(t *testing.T) {
	srv := newTestServer(t)
	defer srv.hub.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status %q, got %q", "ok", body["status"])
	}
}

func TestHandleEvents_Empty(t *testing.T) {
	srv := newTestServer(t)
	defer srv.hub.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var body []any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty array, got %d items", len(body))
	}
}

func TestHandleEvents_WithHistory(t *testing.T) {
	srv := newTestServer(t)
	defer srv.hub.Close()

	// Publish some events directly to the bus's ring buffer
	srv.bus.Publish(events.NewEvent(events.EventUserMessage, events.SourceWS, map[string]any{"content": "hello"}))
	srv.bus.Publish(events.NewEvent(events.EventAssistantMessage, events.SourceAgent, map[string]any{"content": "hi"}))

	waitForEvents(srv.bus, 2)

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var body []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(body))
	}
}

func TestHandleEvents_LimitParam(t *testing.T) {
	srv := newTestServer(t)
	defer srv.hub.Close()

	// Publish 10 events
	for i := 0; i < 10; i++ {
		srv.bus.Publish(events.NewEvent(events.EventUserMessage, events.SourceWS, map[string]any{"i": i}))
	}

	waitForEvents(srv.bus, 10)

	req := httptest.NewRequest(http.MethodGet, "/api/events?limit=5", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var body []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body) != 5 {
		t.Fatalf("expected 5 events with limit=5, got %d", len(body))
	}
}

func TestHandleSessions_Empty(t *testing.T) {
	srv := newTestServer(t)
	defer srv.hub.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var body []any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	// FileStore returns nil for empty dir, which encodes as null
	// The handler just encodes whatever List() returns
}

func TestHandleSessions_WithSessions(t *testing.T) {
	srv := newTestServer(t)
	defer srv.hub.Close()

	// Create sessions via the store
	s1, err := srv.store.Create()
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	s2, err := srv.store.Create()
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	_ = s1
	_ = s2

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var body []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(body))
	}
}
