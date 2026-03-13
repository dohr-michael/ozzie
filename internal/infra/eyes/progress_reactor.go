package eyes

import (
	"context"
	"log/slog"
	"sync"

	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/dohr-michael/ozzie/pkg/connector"
)

// ProgressReactorConfig configures the ProgressReactor.
type ProgressReactorConfig struct {
	Bus        events.EventBus
	Connectors map[string]connector.Connector
	SessionMap *SessionMap
}

// ProgressReactor subscribes to EventBus events and adds/removes
// reactions on the originating connector message to show execution progress.
type ProgressReactor struct {
	connectors map[string]connector.Connector
	sessionMap *SessionMap
	mu         sync.Mutex
	active     map[string][]connector.ReactionType // sessionID → active reactions (dedup)
	unsubs     []func()
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewProgressReactor creates and starts a ProgressReactor.
func NewProgressReactor(cfg ProgressReactorConfig) *ProgressReactor {
	ctx, cancel := context.WithCancel(context.Background())
	pr := &ProgressReactor{
		connectors: cfg.Connectors,
		sessionMap: cfg.SessionMap,
		active:     make(map[string][]connector.ReactionType),
		ctx:        ctx,
		cancel:     cancel,
	}

	pr.unsubs = append(pr.unsubs,
		cfg.Bus.Subscribe(pr.handleLLMCall, events.EventLLMCall),
		cfg.Bus.Subscribe(pr.handleToolCall, events.EventToolCall),
		cfg.Bus.Subscribe(pr.handleDone, events.EventAssistantMessage),
	)

	return pr
}

// Stop unsubscribes from events and cancels in-flight reactions.
func (pr *ProgressReactor) Stop() {
	pr.cancel()
	for _, unsub := range pr.unsubs {
		unsub()
	}
}

func (pr *ProgressReactor) handleLLMCall(e events.Event) {
	if e.SessionID == "" {
		return
	}
	payload, ok := events.GetLLMCallPayload(e)
	if !ok {
		return
	}
	if payload.Phase == "request" {
		pr.addReaction(e.SessionID, connector.ReactionThinking)
	}
}

func (pr *ProgressReactor) handleToolCall(e events.Event) {
	if e.SessionID == "" {
		return
	}
	payload, ok := events.GetToolCallPayload(e)
	if !ok {
		return
	}
	if payload.Status == events.ToolStatusStarted {
		pr.addReaction(e.SessionID, toolReaction(payload.Name))
	}
}

func (pr *ProgressReactor) handleDone(e events.Event) {
	if e.SessionID == "" {
		return
	}
	pr.clearReactions(e.SessionID)
}

func (pr *ProgressReactor) addReaction(sessionID string, rt connector.ReactionType) {
	pr.mu.Lock()
	// Dedup: skip if reaction already active for this session
	for _, r := range pr.active[sessionID] {
		if r == rt {
			pr.mu.Unlock()
			return
		}
	}
	pr.active[sessionID] = append(pr.active[sessionID], rt)
	pr.mu.Unlock()

	connName, channelID, messageID, ok := pr.sessionMap.MessageContext(sessionID)
	if !ok {
		return
	}

	r, ok := pr.resolveReactioner(connName)
	if !ok {
		return
	}

	go func() {
		if err := r.AddReaction(pr.ctx, channelID, messageID, rt); err != nil {
			slog.Debug("progress reaction add failed", "reaction", rt, "error", err)
		}
	}()
}

func (pr *ProgressReactor) clearReactions(sessionID string) {
	pr.mu.Lock()
	reactions := pr.active[sessionID]
	delete(pr.active, sessionID)
	pr.mu.Unlock()

	if len(reactions) == 0 {
		return
	}

	connName, channelID, messageID, ok := pr.sessionMap.MessageContext(sessionID)
	if !ok {
		return
	}

	r, ok := pr.resolveReactioner(connName)
	if !ok {
		return
	}

	go func() {
		for _, rt := range reactions {
			if err := r.RemoveReaction(pr.ctx, channelID, messageID, rt); err != nil {
				slog.Debug("progress reaction remove failed", "reaction", rt, "error", err)
			}
		}
	}()
}

func (pr *ProgressReactor) resolveReactioner(connName string) (connector.Reactioner, bool) {
	c, ok := pr.connectors[connName]
	if !ok {
		return nil, false
	}
	r, ok := c.(connector.Reactioner)
	return r, ok
}

// toolReaction maps a tool name to its semantic ReactionType.
func toolReaction(toolName string) connector.ReactionType {
	switch toolName {
	case "web", "web_search", "web_fetch":
		return connector.ReactionWeb
	case "run_command":
		return connector.ReactionCommand
	case "str_replace_editor":
		return connector.ReactionEdit
	case "submit_task":
		return connector.ReactionTask
	case "store_memory", "forget_memory":
		return connector.ReactionMemory
	case "schedule_task", "trigger_schedule":
		return connector.ReactionSchedule
	case "activate":
		return connector.ReactionActivate
	case "query_tasks", "cancel_task":
		return connector.ReactionTask
	default:
		return connector.ReactionTool
	}
}
