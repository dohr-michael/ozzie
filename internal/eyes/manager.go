package eyes

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/dohr-michael/ozzie/internal/core/policy"
	"github.com/dohr-michael/ozzie/internal/sessions"
	"github.com/dohr-michael/ozzie/pkg/connector"
)

// ManagerConfig configures the ConnectorManager.
type ManagerConfig struct {
	Bus          events.EventBus
	SessionStore sessions.Store
	PairingStore *policy.PairingStore
	SessionMap   *SessionMap
	Connectors   []connector.Connector
}

// Manager bridges the EventBus and external connectors.
// It routes incoming messages to the EventRunner (via EventUserMessage)
// and outgoing assistant messages back to the originating connector.
type Manager struct {
	bus          events.EventBus
	sessionStore sessions.Store
	pairingStore *policy.PairingStore
	sessionMap   *SessionMap
	connectors   map[string]connector.Connector

	// typing tracks active typing indicators per session
	typingMu sync.Mutex
	typing   map[string]func() // sessionID → cancel typing

	ctx    context.Context
	cancel context.CancelFunc
	unsubs []func()
}

// NewManager creates a new connector manager.
func NewManager(cfg ManagerConfig) *Manager {
	connMap := make(map[string]connector.Connector, len(cfg.Connectors))
	for _, c := range cfg.Connectors {
		connMap[c.Name()] = c
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		bus:          cfg.Bus,
		sessionStore: cfg.SessionStore,
		pairingStore: cfg.PairingStore,
		sessionMap:   cfg.SessionMap,
		connectors:   connMap,
		typing:       make(map[string]func()),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start launches all connectors and subscribes to outgoing events.
func (m *Manager) Start() {
	// Subscribe to assistant messages to route responses back
	unsub1 := m.bus.Subscribe(m.handleAssistantMessage, events.EventAssistantMessage)
	m.unsubs = append(m.unsubs, unsub1)

	// Subscribe to stream end to cancel typing
	unsub2 := m.bus.Subscribe(m.handleAssistantStream, events.EventAssistantStream)
	m.unsubs = append(m.unsubs, unsub2)

	// Launch each connector
	for _, c := range m.connectors {
		go m.runConnector(c)
	}
}

// Stop shuts down all connectors and unsubscribes from events.
func (m *Manager) Stop() {
	m.cancel()
	for _, unsub := range m.unsubs {
		unsub()
	}
	for _, c := range m.connectors {
		if err := c.Stop(); err != nil {
			slog.Warn("connector stop error", "connector", c.Name(), "error", err)
		}
	}
	// Cancel all active typing indicators
	m.typingMu.Lock()
	for _, cancel := range m.typing {
		cancel()
	}
	m.typing = nil
	m.typingMu.Unlock()
}

// runConnector starts a connector with auto-reconnect on failure.
func (m *Manager) runConnector(c connector.Connector) {
	for {
		slog.Info("starting connector", "connector", c.Name())
		err := c.Start(m.ctx, func(ctx context.Context, msg connector.IncomingMessage) {
			m.handleIncoming(ctx, c, msg)
		})
		if m.ctx.Err() != nil {
			return // context cancelled, shutting down
		}
		if err != nil {
			slog.Error("connector disconnected", "connector", c.Name(), "error", err)
		}
		// Backoff before reconnect
		select {
		case <-time.After(5 * time.Second):
		case <-m.ctx.Done():
			return
		}
	}
}

// handleIncoming processes a message from a connector.
func (m *Manager) handleIncoming(ctx context.Context, c connector.Connector, msg connector.IncomingMessage) {
	// 1. Publish observability event
	m.bus.Publish(events.NewTypedEvent(events.SourceConnector, events.IncomingMessagePayload{
		Connector: c.Name(),
		ChannelID: msg.ChannelID,
		UserID:    msg.Identity.UserID,
		UserName:  msg.Identity.Name,
		Content:   msg.Content,
		MessageID: msg.MessageID,
	}))

	// 2. Check pairing
	policyName, paired := m.pairingStore.Resolve(msg.Identity)
	if !paired {
		m.handleUnpairedUser(ctx, c, msg)
		return
	}

	// 3. Resolve or create session
	sessionID, err := m.resolveSession(c.Name(), msg.Identity, policyName)
	if err != nil {
		slog.Error("resolve session failed", "connector", c.Name(), "error", err)
		return
	}

	// 4. Start typing indicator
	cancelTyping := c.SendTyping(ctx, msg.ChannelID)
	m.typingMu.Lock()
	m.typing[sessionID] = cancelTyping
	m.typingMu.Unlock()

	// 5. Track message context for progress reactions
	m.sessionMap.SetMessageContext(sessionID, c.Name(), msg.ChannelID, msg.MessageID)

	// 6. Publish user message → EventRunner handles it transparently
	m.bus.Publish(events.NewTypedEventWithSession(events.SourceConnector, events.UserMessagePayload{
		Content: msg.Content,
	}, sessionID))
}

// handleUnpairedUser responds to unknown users and publishes a pairing request.
func (m *Manager) handleUnpairedUser(ctx context.Context, c connector.Connector, msg connector.IncomingMessage) {
	// Reply to the user
	if err := c.Send(ctx, connector.OutgoingMessage{
		Content:   "Your access is pending approval. An administrator has been notified.",
		ChannelID: msg.ChannelID,
		ReplyToID: msg.MessageID,
	}); err != nil {
		slog.Warn("failed to send unpaired reply", "connector", c.Name(), "error", err)
	}

	// Publish pairing request event
	m.bus.Publish(events.NewTypedEvent(events.SourceConnector, events.PairingRequestPayload{
		Platform:  msg.Identity.Platform,
		ServerID:  msg.Identity.ServerID,
		ChannelID: msg.Identity.ChannelID,
		UserID:    msg.Identity.UserID,
		UserName:  msg.Identity.Name,
		Content:   msg.Content,
	}))
}

// resolveSession finds an existing session or creates a new one.
func (m *Manager) resolveSession(connName string, id connector.Identity, policyName string) (string, error) {
	sessionID, ok := m.sessionMap.Get(connName, id.ServerID, id.ChannelID, id.UserID)
	if ok {
		// Verify session still exists and is active
		s, err := m.sessionStore.Get(sessionID)
		if err == nil && s.Status == sessions.SessionActive {
			return sessionID, nil
		}
		// Session gone or closed — create a new one
	}

	// Create new session
	s, err := m.sessionStore.Create()
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	s.PolicyName = policyName
	s.Metadata = map[string]string{
		"connector": connName,
		"platform":  id.Platform,
		"user_id":   id.UserID,
		"user_name": id.Name,
	}
	if err := m.sessionStore.UpdateMeta(s); err != nil {
		return "", fmt.Errorf("update session meta: %w", err)
	}

	if err := m.sessionMap.Set(connName, id.ServerID, id.ChannelID, id.UserID, s.ID); err != nil {
		slog.Warn("persist session map failed", "error", err)
	}

	// Publish session created event
	m.bus.Publish(events.NewEventWithSession(events.EventSessionCreated, events.SourceConnector, map[string]any{
		"connector":   connName,
		"policy_name": policyName,
	}, s.ID))

	return s.ID, nil
}

// handleAssistantMessage routes assistant responses back to the originating connector.
func (m *Manager) handleAssistantMessage(e events.Event) {
	if e.SessionID == "" {
		return
	}
	payload, ok := events.GetAssistantMessagePayload(e)
	if !ok || payload.Content == "" {
		return
	}

	connName, channelID, ok := m.sessionMap.BySession(e.SessionID)
	if !ok {
		return // not a connector session
	}

	// Cancel typing indicator
	m.cancelTyping(e.SessionID)

	c, ok := m.connectors[connName]
	if !ok {
		return
	}

	if err := c.Send(m.ctx, connector.OutgoingMessage{
		Content:   payload.Content,
		ChannelID: channelID,
	}); err != nil {
		slog.Error("send response failed", "connector", connName, "error", err)
	}

	// Publish observability event
	m.bus.Publish(events.NewTypedEventWithSession(events.SourceConnector, events.OutgoingMessagePayload{
		Connector: connName,
		ChannelID: channelID,
		Content:   payload.Content,
	}, e.SessionID))
}

// handleAssistantStream cancels typing on stream end.
func (m *Manager) handleAssistantStream(e events.Event) {
	if e.SessionID == "" {
		return
	}
	payload, ok := events.GetAssistantStreamPayload(e)
	if !ok {
		return
	}
	if payload.Phase == events.StreamPhaseEnd {
		m.cancelTyping(e.SessionID)
	}
}

// ConnectorsByName returns the connector map (used by ProgressReactor).
func (m *Manager) ConnectorsByName() map[string]connector.Connector {
	return m.connectors
}

// cancelTyping stops the typing indicator for a session.
func (m *Manager) cancelTyping(sessionID string) {
	m.typingMu.Lock()
	if cancel, ok := m.typing[sessionID]; ok {
		cancel()
		delete(m.typing, sessionID)
	}
	m.typingMu.Unlock()
}
