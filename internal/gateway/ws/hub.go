package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/coder/websocket"

	"github.com/dohr-michael/ozzie/internal/events"
)

// Client represents a connected WebSocket client.
type Client struct {
	conn *websocket.Conn
	send chan []byte
	hub  *Hub
}

// Hub manages WebSocket clients and bridges them to the event bus.
type Hub struct {
	mu          sync.RWMutex
	clients     map[*Client]struct{}
	bus         *events.Bus
	unsubscribe func()
}

// NewHub creates a new WebSocket hub connected to an event bus.
func NewHub(bus *events.Bus) *Hub {
	h := &Hub{
		clients: make(map[*Client]struct{}),
		bus:     bus,
	}

	// Subscribe to all events and bridge to WS clients
	h.unsubscribe = bus.Subscribe(func(e events.Event) {
		frame, err := NewEventFrame(string(e.Type), e)
		if err != nil {
			slog.Error("marshal event frame", "error", err)
			return
		}
		data, err := MarshalFrame(frame)
		if err != nil {
			slog.Error("marshal frame", "error", err)
			return
		}
		h.broadcast(data)
	})

	return h
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

// register adds a client to the hub.
func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	slog.Info("ws client connected", "clients", len(h.clients))
}

// unregister removes a client from the hub.
func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
		slog.Info("ws client disconnected", "clients", len(h.clients))
	}
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
		c.hub.unregister(c)
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
	switch frame.Method {
	case "send_message":
		var params struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(frame.Params, &params); err != nil {
			c.sendError(ctx, frame.ID, "invalid params")
			return
		}

		c.hub.bus.Publish(events.NewTypedEvent("ws", events.UserMessagePayload{
			Content: params.Content,
		}))

		c.sendOK(ctx, frame.ID, map[string]string{"status": "sent"})

	case "prompt_response":
		var params events.PromptResponsePayload
		if err := json.Unmarshal(frame.Params, &params); err != nil {
			c.sendError(ctx, frame.ID, "invalid params")
			return
		}

		c.hub.bus.Publish(events.NewTypedEvent("ws", params))
		c.sendOK(ctx, frame.ID, map[string]string{"status": "sent"})

	default:
		c.sendError(ctx, frame.ID, "unknown method: "+frame.Method)
	}
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
