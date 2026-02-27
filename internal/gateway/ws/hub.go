package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"filippo.io/age"
	"github.com/coder/websocket"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/plugins"
	"github.com/dohr-michael/ozzie/internal/secrets"
	"github.com/dohr-michael/ozzie/internal/sessions"
)

// Client represents a connected WebSocket client.
type Client struct {
	conn      *websocket.Conn
	send      chan []byte
	hub       *Hub
	sessionID string
}

// TaskHandler provides task operations for WS methods.
type TaskHandler interface {
	Submit(sessionID string, title, description string, tools []string, priority string) (string, error)
	Check(taskID string) (any, error)
	Cancel(taskID string, reason string) error
	List(sessionID string) (any, error)
	ReplyTask(taskID string, feedback string, status string, sessionID string) error
}

// Hub manages WebSocket clients and bridges them to the event bus.
type Hub struct {
	mu             sync.RWMutex
	clients        map[*Client]struct{}
	bus            *events.Bus
	store          sessions.Store
	tasks          TaskHandler
	perms          *plugins.ToolPermissions
	unsubscribe    func()
	recipient      *age.X25519Recipient // nil = encryption disabled
	passwordTokens sync.Map             // token → bool
}

// NewHub creates a new WebSocket hub connected to an event bus.
func NewHub(bus *events.Bus, store sessions.Store, perms *plugins.ToolPermissions) *Hub {
	h := &Hub{
		clients: make(map[*Client]struct{}),
		bus:     bus,
		store:   store,
		perms:   perms,
	}

	// Track password prompt tokens for encryption
	bus.Subscribe(func(e events.Event) {
		if p, ok := events.GetPromptRequestPayload(e); ok && p.Type == events.PromptTypePassword {
			h.passwordTokens.Store(p.Token, true)
		}
	}, events.EventPromptRequest)

	// Subscribe to all events and bridge to WS clients
	h.unsubscribe = bus.Subscribe(func(e events.Event) {
		frame, err := NewEventFrame(string(e.Type), e.SessionID, e)
		if err != nil {
			slog.Error("marshal event frame", "error", err)
			return
		}
		data, err := MarshalFrame(frame)
		if err != nil {
			slog.Error("marshal frame", "error", err)
			return
		}

		if e.SessionID != "" {
			h.sendToSession(e.SessionID, data)
		} else {
			h.broadcast(data)
		}
	})

	return h
}

// SetTaskHandler sets the optional task handler for WS task methods.
func (h *Hub) SetTaskHandler(th TaskHandler) {
	h.tasks = th
}

// SetSecretEncryptor enables encryption for password prompt responses.
func (h *Hub) SetSecretEncryptor(r *age.X25519Recipient) {
	h.recipient = r
}

// broadcast sends data to all connected clients.
func (h *Hub) broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// Client too slow, skip
		}
	}
}

// sendToSession sends data only to clients in a specific session.
func (h *Hub) sendToSession(sessionID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		if c.sessionID == sessionID {
			select {
			case c.send <- data:
			default:
			}
		}
	}
}

// register adds a client to the hub.
func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	slog.Info("ws client connected", "clients", len(h.clients))
}

// unregisterClient removes a client from the hub and closes its session if it was the last client.
func (h *Hub) unregisterClient(c *Client) {
	h.mu.Lock()

	if _, ok := h.clients[c]; !ok {
		h.mu.Unlock()
		return
	}

	sessionID := c.sessionID
	delete(h.clients, c)
	close(c.send)
	slog.Info("ws client disconnected", "clients", len(h.clients), "session_id", sessionID)

	// Check if this was the last client in the session
	if sessionID != "" {
		lastClient := true
		for other := range h.clients {
			if other.sessionID == sessionID {
				lastClient = false
				break
			}
		}
		h.mu.Unlock()

		if lastClient {
			if err := h.store.Close(sessionID); err != nil {
				slog.Error("close session", "error", err, "session_id", sessionID)
			}
			// NOTE: do NOT cleanup tool permissions here — async tasks
			// may still be running with this session's accept-all flag.
			// Permissions are lightweight and scoped to the gateway lifetime.
			h.bus.Publish(events.NewEventWithSession(
				events.EventSessionClosed, events.SourceHub,
				map[string]any{"session_id": sessionID}, sessionID,
			))
		}
	} else {
		h.mu.Unlock()
	}
}

// handleOpenSession creates or resumes a session for the client.
func (h *Hub) handleOpenSession(c *Client, frameID string, sessionID string, rootDir string) {
	ctx := context.Background()

	if sessionID != "" {
		// Resume existing session
		s, err := h.store.Get(sessionID)
		if err != nil {
			c.sendError(ctx, frameID, "session not found: "+sessionID)
			return
		}
		c.sessionID = s.ID

		// Update root_dir if provided (client may have changed directory)
		if rootDir != "" && rootDir != s.RootDir {
			s.RootDir = rootDir
			_ = h.store.UpdateMeta(s)
		}

		c.sendOK(ctx, frameID, map[string]string{
			"session_id": s.ID,
			"status":     "resumed",
		})
		return
	}

	// Create new session
	s, err := h.store.Create()
	if err != nil {
		c.sendError(ctx, frameID, "create session: "+err.Error())
		return
	}

	c.sessionID = s.ID

	// Store root_dir if provided
	if rootDir != "" {
		s.RootDir = rootDir
		_ = h.store.UpdateMeta(s)
	}

	h.bus.Publish(events.NewEventWithSession(
		events.EventSessionCreated, events.SourceHub,
		map[string]any{"session_id": s.ID}, s.ID,
	))

	c.sendOK(ctx, frameID, map[string]string{
		"session_id": s.ID,
		"status":     "created",
	})
}

// ensureSession auto-creates a session for a client if it doesn't have one.
func (h *Hub) ensureSession(c *Client) {
	if c.sessionID != "" {
		return
	}

	s, err := h.store.Create()
	if err != nil {
		slog.Error("auto-create session", "error", err)
		return
	}

	c.sessionID = s.ID
	slog.Info("auto-created session", "session_id", s.ID)

	h.bus.Publish(events.NewEventWithSession(
		events.EventSessionCreated, events.SourceHub,
		map[string]any{"session_id": s.ID}, s.ID,
	))
}

// ServeWS handles a WebSocket upgrade and manages the client lifecycle.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow any origin for dev
	})
	if err != nil {
		slog.Error("ws accept", "error", err)
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan []byte, 256),
		hub:  h,
	}

	h.register(client)

	ctx := r.Context()
	go client.writePump(ctx)
	client.readPump(ctx)
}

// readPump reads frames from the WS connection and dispatches them.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.unregisterClient(c)
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != -1 {
				slog.Debug("ws read closed", "status", websocket.CloseStatus(err))
			} else {
				slog.Debug("ws read error", "error", err)
			}
			return
		}

		frame, err := UnmarshalFrame(data)
		if err != nil {
			slog.Error("ws unmarshal frame", "error", err)
			continue
		}

		c.handleFrame(ctx, frame)
	}
}

// handleFrame processes an incoming WS frame.
func (c *Client) handleFrame(ctx context.Context, frame Frame) {
	switch frame.Type {
	case FrameTypeRequest:
		c.handleRequest(ctx, frame)
	default:
		slog.Debug("ws unknown frame type", "type", frame.Type)
	}
}

// handleRequest processes a request frame (method dispatch).
func (c *Client) handleRequest(ctx context.Context, frame Frame) {
	switch Method(frame.Method) {
	case MethodOpenSession:
		var params struct {
			SessionID string `json:"session_id"`
			RootDir   string `json:"root_dir"`
		}
		if frame.Params != nil {
			if err := json.Unmarshal(frame.Params, &params); err != nil {
				c.sendError(ctx, frame.ID, "invalid params")
				return
			}
		}
		c.hub.handleOpenSession(c, frame.ID, params.SessionID, params.RootDir)

	case MethodSendMessage:
		var params struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(frame.Params, &params); err != nil {
			c.sendError(ctx, frame.ID, "invalid params")
			return
		}

		// Auto-create session if needed
		c.hub.ensureSession(c)

		c.hub.bus.Publish(events.NewTypedEventWithSession(events.SourceWS, events.UserMessagePayload{
			Content: params.Content,
		}, c.sessionID))

		c.sendOK(ctx, frame.ID, map[string]string{"status": "sent"})

	case MethodPromptResponse:
		var params events.PromptResponsePayload
		if err := json.Unmarshal(frame.Params, &params); err != nil {
			c.sendError(ctx, frame.ID, "invalid params")
			return
		}

		// Encrypt password responses before they reach the event bus / LLM
		if _, ok := c.hub.passwordTokens.LoadAndDelete(params.Token); ok && c.hub.recipient != nil {
			if strVal, ok := params.Value.(string); ok {
				encrypted, err := secrets.Encrypt(strVal, c.hub.recipient)
				if err != nil {
					c.sendError(ctx, frame.ID, "encrypt secret: "+err.Error())
					return
				}
				params.Value = encrypted
			}
		}

		c.hub.bus.Publish(events.NewTypedEventWithSession(events.SourceWS, params, c.sessionID))
		c.sendOK(ctx, frame.ID, map[string]string{"status": "sent"})

	case MethodSubmitTask:
		c.handleSubmitTask(ctx, frame)

	case MethodCheckTask:
		c.handleCheckTask(ctx, frame)

	case MethodCancelTask:
		c.handleCancelTask(ctx, frame)

	case MethodListTasks:
		c.handleListTasks(ctx, frame)

	case MethodReplyTask:
		c.handleReplyTask(ctx, frame)

	case MethodAcceptAllTools:
		c.hub.ensureSession(c)
		if c.hub.perms != nil && c.sessionID != "" {
			c.hub.perms.AllowAllForSession(c.sessionID)
		}
		c.sendOK(ctx, frame.ID, map[string]string{"status": "accepted"})

	default:
		c.sendError(ctx, frame.ID, "unknown method: "+frame.Method)
	}
}

func (c *Client) handleSubmitTask(ctx context.Context, frame Frame) {
	if c.hub.tasks == nil {
		c.sendError(ctx, frame.ID, "task system not available")
		return
	}

	var params struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Tools       []string `json:"tools"`
		Priority    string   `json:"priority"`
	}
	if err := json.Unmarshal(frame.Params, &params); err != nil {
		c.sendError(ctx, frame.ID, "invalid params")
		return
	}

	c.hub.ensureSession(c)

	taskID, err := c.hub.tasks.Submit(c.sessionID, params.Title, params.Description, params.Tools, params.Priority)
	if err != nil {
		c.sendError(ctx, frame.ID, err.Error())
		return
	}

	c.sendOK(ctx, frame.ID, map[string]string{"task_id": taskID, "status": "submitted"})
}

func (c *Client) handleCheckTask(ctx context.Context, frame Frame) {
	if c.hub.tasks == nil {
		c.sendError(ctx, frame.ID, "task system not available")
		return
	}

	var params struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(frame.Params, &params); err != nil {
		c.sendError(ctx, frame.ID, "invalid params")
		return
	}

	result, err := c.hub.tasks.Check(params.TaskID)
	if err != nil {
		c.sendError(ctx, frame.ID, err.Error())
		return
	}

	c.sendOK(ctx, frame.ID, result)
}

func (c *Client) handleCancelTask(ctx context.Context, frame Frame) {
	if c.hub.tasks == nil {
		c.sendError(ctx, frame.ID, "task system not available")
		return
	}

	var params struct {
		TaskID string `json:"task_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(frame.Params, &params); err != nil {
		c.sendError(ctx, frame.ID, "invalid params")
		return
	}

	if err := c.hub.tasks.Cancel(params.TaskID, params.Reason); err != nil {
		c.sendError(ctx, frame.ID, err.Error())
		return
	}

	c.sendOK(ctx, frame.ID, map[string]string{"task_id": params.TaskID, "status": "cancelled"})
}

func (c *Client) handleListTasks(ctx context.Context, frame Frame) {
	if c.hub.tasks == nil {
		c.sendError(ctx, frame.ID, "task system not available")
		return
	}

	result, err := c.hub.tasks.List(c.sessionID)
	if err != nil {
		c.sendError(ctx, frame.ID, err.Error())
		return
	}

	c.sendOK(ctx, frame.ID, result)
}

func (c *Client) handleReplyTask(ctx context.Context, frame Frame) {
	if c.hub.tasks == nil {
		c.sendError(ctx, frame.ID, "task system not available")
		return
	}

	var params struct {
		TaskID   string `json:"task_id"`
		Feedback string `json:"feedback"`
		Status   string `json:"status"` // "approved" | "revise"
	}
	if err := json.Unmarshal(frame.Params, &params); err != nil {
		c.sendError(ctx, frame.ID, "invalid params")
		return
	}

	// Default to "approved" for backwards compatibility
	if params.Status == "" {
		params.Status = "approved"
	}

	c.hub.ensureSession(c)

	if err := c.hub.tasks.ReplyTask(params.TaskID, params.Feedback, params.Status, c.sessionID); err != nil {
		c.sendError(ctx, frame.ID, err.Error())
		return
	}

	c.sendOK(ctx, frame.ID, map[string]string{"task_id": params.TaskID, "status": "replied"})
}

// writePump writes queued messages to the WS connection.
func (c *Client) writePump(ctx context.Context) {
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.conn.Write(ctx, websocket.MessageText, msg); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) sendOK(ctx context.Context, id string, payload any) {
	f, err := NewResponseFrame(id, true, payload, "")
	if err != nil {
		return
	}
	data, err := MarshalFrame(f)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (c *Client) sendError(ctx context.Context, id string, errMsg string) {
	f, err := NewResponseFrame(id, false, nil, errMsg)
	if err != nil {
		return
	}
	data, err := MarshalFrame(f)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

// Close shuts down the hub and all client connections.
func (h *Hub) Close() {
	if h.unsubscribe != nil {
		h.unsubscribe()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		c.conn.Close(websocket.StatusGoingAway, "server shutdown")
		delete(h.clients, c)
	}
}
