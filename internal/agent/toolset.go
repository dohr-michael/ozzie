package agent

import (
	"sort"
	"sync"
)

// ToolSet tracks per-session active tools. Core tools are always active;
// additional tools can be activated at runtime via activate_tools.
// All methods are safe for concurrent use.
type ToolSet struct {
	mu        sync.RWMutex
	core      map[string]bool            // always-on tools
	active    map[string]map[string]bool // sessionID → tool names
	turnFlags map[string]bool            // sessionID → activated-during-turn
	allNames  map[string]bool            // every known tool name
}

// NewToolSet creates a ToolSet. coreTools are always active for every session.
// allTools is the full catalog of known tool names.
func NewToolSet(coreTools, allTools []string) *ToolSet {
	core := make(map[string]bool, len(coreTools))
	for _, n := range coreTools {
		core[n] = true
	}
	all := make(map[string]bool, len(allTools))
	for _, n := range allTools {
		all[n] = true
	}
	return &ToolSet{
		core:      core,
		active:    make(map[string]map[string]bool),
		turnFlags: make(map[string]bool),
		allNames:  all,
	}
}

// ActiveToolNames returns the sorted list of tool names currently active for
// the given session (core + session-specific activations).
func (ts *ToolSet) ActiveToolNames(sessionID string) []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	set := make(map[string]bool, len(ts.core))
	for n := range ts.core {
		set[n] = true
	}
	for n := range ts.active[sessionID] {
		set[n] = true
	}

	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Activate adds a tool to the session's active set. Returns false if the tool
// name is not in the known catalog.
func (ts *ToolSet) Activate(sessionID, toolName string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if !ts.allNames[toolName] {
		return false
	}
	if ts.active[sessionID] == nil {
		ts.active[sessionID] = make(map[string]bool)
	}
	ts.active[sessionID][toolName] = true
	ts.turnFlags[sessionID] = true
	return true
}

// IsKnown returns true if toolName is in the full catalog.
func (ts *ToolSet) IsKnown(toolName string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.allNames[toolName]
}

// IsActive returns true if the tool is currently active for the session
// (either core or explicitly activated).
func (ts *ToolSet) IsActive(sessionID, toolName string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if ts.core[toolName] {
		return true
	}
	return ts.active[sessionID][toolName]
}

// ResetTurnFlag clears the activation flag for the current turn.
func (ts *ToolSet) ResetTurnFlag(sessionID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.turnFlags[sessionID] = false
}

// ActivatedDuringTurn returns true if any tool was activated since the last
// ResetTurnFlag call for this session.
func (ts *ToolSet) ActivatedDuringTurn(sessionID string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.turnFlags[sessionID]
}

// HasInactiveTools returns true if there are known tools that are not currently
// active for the session.
func (ts *ToolSet) HasInactiveTools(sessionID string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	active := len(ts.core)
	for range ts.active[sessionID] {
		active++
	}
	return active < len(ts.allNames)
}

// Cleanup removes all per-session state for the given session.
func (ts *ToolSet) Cleanup(sessionID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.active, sessionID)
	delete(ts.turnFlags, sessionID)
}
