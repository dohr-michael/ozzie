package connectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type sessionKey struct {
	Connector string `json:"connector"`
	ServerID  string `json:"server_id"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
}

type sessionEntry struct {
	Key       sessionKey `json:"key"`
	SessionID string     `json:"session_id"`
}

// messageContext tracks the latest user message per session for reaction targeting.
type messageContext struct {
	ConnectorName string
	ChannelID     string
	MessageID     string
}

// SessionMap provides persistent connector→session mapping.
// Thread-safe; persisted as a JSON file.
type SessionMap struct {
	mu      sync.RWMutex
	entries []sessionEntry
	path    string
	msgCtx  map[string]messageContext // sessionID → latest message context (ephemeral)
}

// NewSessionMap creates a session map that persists to dir/connector_sessions.json.
func NewSessionMap(dir string) *SessionMap {
	m := &SessionMap{
		path:   filepath.Join(dir, "connector_sessions.json"),
		msgCtx: make(map[string]messageContext),
	}
	_ = m.load() // best-effort
	return m
}

// Get returns the session ID for the given connector/server/channel/user, or false.
func (m *SessionMap) Get(connector, serverID, channelID, userID string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := sessionKey{Connector: connector, ServerID: serverID, ChannelID: channelID, UserID: userID}
	for _, e := range m.entries {
		if e.Key == key {
			return e.SessionID, true
		}
	}
	return "", false
}

// Set stores the session mapping and persists to disk.
func (m *SessionMap) Set(connector, serverID, channelID, userID, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := sessionKey{Connector: connector, ServerID: serverID, ChannelID: channelID, UserID: userID}
	for i, e := range m.entries {
		if e.Key == key {
			m.entries[i].SessionID = sessionID
			return m.save()
		}
	}
	m.entries = append(m.entries, sessionEntry{Key: key, SessionID: sessionID})
	return m.save()
}

// BySession performs a reverse lookup: given a session ID, returns the connector
// name and channel ID. Returns ok=false if not found.
func (m *SessionMap) BySession(sessionID string) (connector, channelID string, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, e := range m.entries {
		if e.SessionID == sessionID {
			return e.Key.Connector, e.Key.ChannelID, true
		}
	}
	return "", "", false
}

// SetMessageContext tracks the latest user message for a session (ephemeral, not persisted).
func (m *SessionMap) SetMessageContext(sessionID, connectorName, channelID, messageID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgCtx[sessionID] = messageContext{
		ConnectorName: connectorName,
		ChannelID:     channelID,
		MessageID:     messageID,
	}
}

// MessageContext returns the latest message context for a session.
func (m *SessionMap) MessageContext(sessionID string) (connectorName, channelID, messageID string, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ctx, found := m.msgCtx[sessionID]
	if !found {
		return "", "", "", false
	}
	return ctx.ConnectorName, ctx.ChannelID, ctx.MessageID, true
}

func (m *SessionMap) load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m.entries)
}

func (m *SessionMap) save() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return fmt.Errorf("create session map dir: %w", err)
	}
	data, err := json.MarshalIndent(m.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session map: %w", err)
	}
	return os.WriteFile(m.path, data, 0o644)
}
