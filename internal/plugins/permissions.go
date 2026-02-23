package plugins

import "sync"

// ToolPermissions tracks which dangerous tools are auto-approved.
// It supports two levels: global (from config, always approved) and
// per-session (approved dynamically by the client).
type ToolPermissions struct {
	mu             sync.RWMutex
	globalAllowed  map[string]bool            // from config, always approved
	sessionAllowed map[string]map[string]bool // sessionID → tool name → allowed ("*" = accept-all)
}

// NewToolPermissions creates a ToolPermissions with the given globally allowed tool names.
func NewToolPermissions(globalAllowed []string) *ToolPermissions {
	ga := make(map[string]bool, len(globalAllowed))
	for _, name := range globalAllowed {
		ga[name] = true
	}
	return &ToolPermissions{
		globalAllowed:  ga,
		sessionAllowed: make(map[string]map[string]bool),
	}
}

// IsAllowed returns true if the tool is auto-approved for the given session.
// A tool is allowed if it's in the global list, the session's per-tool list,
// or the session has accept-all mode enabled.
func (tp *ToolPermissions) IsAllowed(sessionID, toolName string) bool {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	if tp.globalAllowed[toolName] {
		return true
	}

	sess := tp.sessionAllowed[sessionID]
	if sess == nil {
		return false
	}

	return sess[toolName] || sess["*"]
}

// AllowForSession marks a specific tool as approved for the given session.
func (tp *ToolPermissions) AllowForSession(sessionID, toolName string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.sessionAllowed[sessionID] == nil {
		tp.sessionAllowed[sessionID] = make(map[string]bool)
	}
	tp.sessionAllowed[sessionID][toolName] = true
}

// AllowAllForSession enables accept-all mode for the session (--dangerously-accept-all).
func (tp *ToolPermissions) AllowAllForSession(sessionID string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.sessionAllowed[sessionID] == nil {
		tp.sessionAllowed[sessionID] = make(map[string]bool)
	}
	tp.sessionAllowed[sessionID]["*"] = true
}

// IsSessionAcceptAll returns true if the session has accept-all mode enabled.
func (tp *ToolPermissions) IsSessionAcceptAll(sessionID string) bool {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	sess := tp.sessionAllowed[sessionID]
	return sess != nil && sess["*"]
}

// CleanupSession removes all per-session permissions.
func (tp *ToolPermissions) CleanupSession(sessionID string) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	delete(tp.sessionAllowed, sessionID)
}
