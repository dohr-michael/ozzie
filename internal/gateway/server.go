package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"filippo.io/age"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/gateway/ws"
	"github.com/dohr-michael/ozzie/internal/plugins"
	"github.com/dohr-michael/ozzie/internal/sessions"
)

// Server is the Ozzie gateway HTTP server.
type Server struct {
	httpServer   *http.Server
	hub          *ws.Hub
	bus          *events.Bus
	store        sessions.Store
	taskHandler  *WSTaskHandler
	host         string
	port         int
}

// NewServer creates a new gateway server.
func NewServer(bus *events.Bus, store sessions.Store, host string, port int, perms *plugins.ToolPermissions) *Server {
	hub := ws.NewHub(bus, store, perms)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	s := &Server{
		hub:   hub,
		bus:   bus,
		store: store,
		host:  host,
		port:  port,
	}

	// Routes
	r.Get("/api/health", s.handleHealth)
	r.Get("/api/ws", hub.ServeWS)
	r.Get("/api/events", s.handleEvents)
	r.Get("/api/sessions", s.handleSessions)

	// API: tasks
	r.Get("/api/tasks", s.handleTasks)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", host, port),
		Handler: r,
	}

	return s
}

// Start begins listening. It blocks until the server is stopped.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return err
	}
	slog.Info("Ozzie gateway listening", "addr", ln.Addr().String())
	return s.httpServer.Serve(ln)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.hub.Close()
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	history := s.bus.History(limit)

	w.Header().Set("Content-Type", "application/json")

	// Format timestamps nicely
	type eventJSON struct {
		ID        string         `json:"id"`
		SessionID string         `json:"session_id,omitempty"`
		Type      string         `json:"type"`
		Timestamp string         `json:"timestamp"`
		Source    events.EventSource `json:"source"`
		Payload   map[string]any `json:"payload"`
	}

	result := make([]eventJSON, len(history))
	for i, e := range history {
		result[i] = eventJSON{
			ID:        e.ID,
			SessionID: e.SessionID,
			Type:      string(e.Type),
			Timestamp: e.Timestamp.Format(time.RFC3339Nano),
			Source:    e.Source,
			Payload:   e.Payload,
		}
	}

	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if s.taskHandler == nil {
		http.Error(w, "task system not available", http.StatusServiceUnavailable)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	result, err := s.taskHandler.List(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// SetTaskHandler configures the task handler for WS and HTTP task operations.
func (s *Server) SetTaskHandler(th *WSTaskHandler) {
	s.taskHandler = th
	s.hub.SetTaskHandler(th)
}

// SetSecretEncryptor enables encryption for password prompt responses.
func (s *Server) SetSecretEncryptor(r *age.X25519Recipient) {
	s.hub.SetSecretEncryptor(r)
}
