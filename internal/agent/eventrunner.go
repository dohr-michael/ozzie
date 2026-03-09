package agent

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/layered"
	"github.com/dohr-michael/ozzie/internal/sessions"
)

// ToolLookup resolves tool subsets from the registry.
type ToolLookup interface {
	ToolsByNames(names []string) []tool.InvokableTool
}

// CapacitySlot represents an acquired LLM capacity slot.
// Implemented by actors.Actor.
type CapacitySlot interface{}

// CapacityPool acquires and releases LLM capacity slots.
// Implemented by actors.ActorPool.
type CapacityPool interface {
	AcquireInteractive(providerName string) (CapacitySlot, error)
	Release(slot CapacitySlot)
}

// EventRunner wraps an AgentFactory and provides event-driven execution
// with dynamic tool selection.
type EventRunner struct {
	factory    *AgentFactory
	toolSet    *ToolSet
	registry   ToolLookup
	bus        *events.Bus
	store      sessions.Store
	compressor *Compressor
	layered    *layered.Manager

	pool            CapacityPool // actor pool for capacity management (optional)
	defaultProvider string       // default provider name for AcquireInteractive

	mu           sync.Mutex
	running      map[string]bool // per-session lock
	streamSeqIdx int32

	ctx         context.Context
	cancel      context.CancelFunc
	unsubscribe func()
}

// EventRunnerConfig contains configuration for the EventRunner.
type EventRunnerConfig struct {
	Factory         *AgentFactory
	ToolSet         *ToolSet
	Registry        ToolLookup
	EventBus        *events.Bus
	Store           sessions.Store
	Pool            CapacityPool     // actor pool for capacity management (optional)
	DefaultProvider string           // default provider name for AcquireInteractive
	ContextWindow   int              // total context window in tokens (for compression)
	Tier            ModelTier        // model tier for adaptive compression
	Layered         *layered.Manager // layered context manager (optional)
}

// NewEventRunner creates a new event-driven runner.
func NewEventRunner(cfg EventRunnerConfig) *EventRunner {
	ctx, cancel := context.WithCancel(context.Background())

	compCfg := CompressorConfig{ContextWindow: cfg.ContextWindow}
	if cfg.Tier == TierSmall {
		compCfg.Threshold = 0.60
		compCfg.PreserveRatio = 0.40
	}

	er := &EventRunner{
		factory:         cfg.Factory,
		toolSet:         cfg.ToolSet,
		registry:        cfg.Registry,
		bus:             cfg.EventBus,
		store:           cfg.Store,
		compressor:      NewCompressor(compCfg),
		layered:         cfg.Layered,
		pool:            cfg.Pool,
		defaultProvider: cfg.DefaultProvider,
		running:         make(map[string]bool),
		ctx:             ctx,
		cancel:          cancel,
	}

	er.unsubscribe = cfg.EventBus.Subscribe(er.handleEvent,
		events.EventUserMessage,
		events.EventTaskCompleted,
		events.EventToolCall,
	)

	return er
}

func (er *EventRunner) handleEvent(event events.Event) {
	switch event.Type {
	case events.EventUserMessage:
		if payload, ok := events.GetUserMessagePayload(event); ok && payload.Content != "" {
			go er.processMessage(event.SessionID, payload.Content)
		}
	case events.EventTaskCompleted:
		if payload, ok := events.GetTaskCompletedPayload(event); ok && event.SessionID != "" {
			go er.handleTaskCompleted(event.SessionID, payload)
		}
	case events.EventToolCall:
		if payload, ok := events.GetToolCallPayload(event); ok && event.SessionID != "" {
			er.persistToolLog(event.SessionID, payload)
		}
	}
}

func (er *EventRunner) processMessage(sessionID string, content string) {
	er.mu.Lock()
	if er.running[sessionID] {
		er.mu.Unlock()
		return
	}
	er.running[sessionID] = true
	atomic.StoreInt32(&er.streamSeqIdx, 0)
	er.mu.Unlock()

	defer func() {
		er.mu.Lock()
		delete(er.running, sessionID)
		er.mu.Unlock()
	}()

	// Acquire a capacity slot from the actor pool (if configured)
	if er.pool != nil {
		slot, err := er.pool.AcquireInteractive(er.defaultProvider)
		if err != nil {
			slog.Error("acquire interactive slot", "error", err, "session_id", sessionID)
			er.emitError(sessionID, "All LLM capacity is currently in use. Please try again shortly.")
			return
		}
		defer er.pool.Release(slot)
	}

	// Persist user message
	userMsg := sessions.Message{Role: string(schema.User), Content: content, Ts: time.Now()}
	if err := er.store.AppendMessage(sessionID, userMsg); err != nil {
		slog.Error("persist user message", "error", err, "session_id", sessionID)
	}

	// Load full history
	history, err := er.store.LoadMessages(sessionID)
	if err != nil {
		slog.Error("load messages", "error", err, "session_id", sessionID)
		er.emitError(sessionID, "failed to load session history")
		return
	}

	messages := make([]*schema.Message, 0, len(history))
	for _, m := range history {
		// Skip tool log entries — they are for TUI history display only.
		if m.Role == sessions.RoleToolLog {
			continue
		}
		msg := m.ToSchemaMessage()
		// Skip messages with empty content — APIs reject these.
		if msg.Content == "" {
			continue
		}
		// Merge consecutive user messages to avoid confusing the LLM.
		// This happens when previous responses were empty (not persisted).
		if len(messages) > 0 && messages[len(messages)-1].Role == schema.User && msg.Role == schema.User {
			messages[len(messages)-1].Content += "\n" + msg.Content
			continue
		}
		messages = append(messages, msg)
	}

	// Context compression — layered context takes priority, compressor is fallback.
	compressed := false
	if er.layered != nil {
		if lmsgs, layeredErr := er.layered.Apply(er.ctx, sessionID, messages, history); layeredErr == nil {
			messages = lmsgs
			compressed = true
		} else {
			slog.Warn("layered context failed, falling back to compressor", "error", layeredErr)
		}
	}
	if !compressed {
		if session, err := er.store.Get(sessionID); err == nil {
			sysEstimate := er.compressor.EstimateTokens([]*schema.Message{{
				Role:    schema.System,
				Content: er.factory.Persona(),
			}})

			result, compErr := er.compressor.Compress(er.ctx, session, messages, sysEstimate, er.summarize)
			if compErr != nil {
				slog.Error("context compression", "error", compErr)
			} else {
				messages = result.Messages
				if result.Compressed {
					session.Summary = result.NewSummary
					session.SummaryUpTo = result.NewSummaryUpTo
					_ = er.store.UpdateMeta(session)
				}
			}
		}
	}

	// Retry loop: up to 3 attempts (initial buffered + retry after tool activation + streaming)
	for attempt := 0; attempt < 3; attempt++ {
		activeNames := er.toolSet.ActiveToolNames(sessionID)
		tools := er.registry.ToolsByNames(activeNames)

		er.toolSet.ResetTurnFlag(sessionID)

		// First attempt with inactive tools: run buffered (non-streaming) to detect activation
		if attempt == 0 && er.toolSet.HasInactiveTools(sessionID) {
			runner, err := er.factory.CreateRunnerBuffered(er.ctx, tools)
			if err != nil {
				slog.Error("create runner (buffered)", "error", err, "session_id", sessionID)
				er.emitError(sessionID, "failed to create agent runner")
				return
			}

			content, runErr := er.runAgentBuffered(sessionID, runner, messages)

			if er.toolSet.ActivatedDuringTurn(sessionID) {
				// Tools were activated — retry with expanded tool set (streamed).
				// Any error from the buffered run is expected (the newly activated
				// tool wasn't in the frozen Eino graph yet).
				slog.Info("tools activated, retrying with expanded set",
					"session_id", sessionID)
				continue
			}

			// No explicit activation — check if the LLM tried to call an
			// inactive tool directly (without activate_tools first).
			// If so, auto-activate the tool and retry.
			if runErr != nil {
				if toolName := extractMissingToolName(runErr.Error()); toolName != "" {
					if er.toolSet.IsKnown(toolName) {
						er.toolSet.Activate(sessionID, toolName)
						slog.Info("auto-activated tool called by LLM",
							"tool", toolName,
							"session_id", sessionID)
						continue
					}
				}
				er.emitError(sessionID, runErr.Error())
				return
			}

			// Empty response from buffered run — retry with streaming.
			// Some models return empty in buffered mode; streaming often succeeds.
			if content == "" {
				slog.Warn("empty buffered response, retrying with streaming",
					"session_id", sessionID)
				continue
			}

			// Has content — emit the buffered response.
			er.emitStreamStart(sessionID)
			er.emitStreamDelta(sessionID, content)
			er.emitStreamEnd(sessionID)
			er.persistAndEmitResponse(sessionID, content)
			return
		}

		// All tools active OR retry: stream normally
		runner, err := er.factory.CreateRunner(er.ctx, tools)
		if err != nil {
			slog.Error("create runner", "error", err, "session_id", sessionID)
			er.emitError(sessionID, "failed to create agent runner")
			return
		}

		er.emitStreamStart(sessionID)
		er.runAgent(sessionID, runner, messages)
		return
	}
}

func (er *EventRunner) runAgent(sessionID string, runner *adk.Runner, messages []*schema.Message) {
	ctx := events.ContextWithSessionID(er.ctx, sessionID)
	ctx = er.withSessionWorkDir(ctx, sessionID)
	checkpointID := uuid.New().String()
	iter := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))
	er.consumeIterator(sessionID, iter)
}

func (er *EventRunner) runAgentBuffered(sessionID string, runner *adk.Runner, messages []*schema.Message) (string, error) {
	ctx := events.ContextWithSessionID(er.ctx, sessionID)
	ctx = er.withSessionWorkDir(ctx, sessionID)
	checkpointID := uuid.New().String()
	iter := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))
	return er.consumeIteratorBuffered(sessionID, iter)
}

// withSessionContext propagates the session's RootDir as WorkDir and
// ToolConstraints into the context.
func (er *EventRunner) withSessionWorkDir(ctx context.Context, sessionID string) context.Context {
	sess, err := er.store.Get(sessionID)
	if err != nil {
		return ctx
	}
	if sess.RootDir != "" {
		ctx = events.ContextWithWorkDir(ctx, sess.RootDir)
	}
	if len(sess.ToolConstraints) > 0 {
		ctx = events.ContextWithToolConstraints(ctx, sess.ToolConstraints)
	}
	return ctx
}

func (er *EventRunner) consumeIterator(sessionID string, iter *adk.AsyncIterator[*adk.AgentEvent]) {
	content, _ := ConsumeIterator(iter, IterCallbacks{
		OnStreamChunk: func(chunk string) { er.emitStreamDelta(sessionID, chunk) },
		OnStreamDone:  func() { er.emitStreamEnd(sessionID) },
		OnError: func(err error) {
			slog.Error("agent error", "error", err)
			er.emitError(sessionID, err.Error())
		},
	})

	// Always emit StreamEnd to match the StreamStart emitted before runAgent.
	// This ensures the TUI resets its streaming state even on empty responses.
	if content == "" {
		er.emitStreamEnd(sessionID)
	}

	er.persistAndEmitResponse(sessionID, content)
}

func (er *EventRunner) consumeIteratorBuffered(_ string, iter *adk.AsyncIterator[*adk.AgentEvent]) (string, error) {
	return ConsumeIterator(iter, IterCallbacks{})
}

func (er *EventRunner) persistAndEmitResponse(sessionID string, content string) {
	if content != "" {
		assistantMsg := sessions.Message{Role: string(schema.Assistant), Content: content, Ts: time.Now()}
		if err := er.store.AppendMessage(sessionID, assistantMsg); err != nil {
			slog.Error("persist assistant message", "error", err, "session_id", sessionID)
		}
	}

	er.emitResponse(sessionID, content)
}

// summarize performs a non-streaming LLM call for summarization (no tools).
func (er *EventRunner) summarize(ctx context.Context, prompt string) (string, error) {
	runner, err := er.factory.CreateRunnerBuffered(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("create summarization runner: %w", err)
	}

	messages := []*schema.Message{{Role: schema.User, Content: prompt}}
	iter := runner.Run(ctx, messages)

	content, err := ConsumeIterator(iter, IterCallbacks{})
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", fmt.Errorf("empty summarization response")
	}
	return content, nil
}

// handleTaskCompleted appends a system message to the session so the user sees
// the notification on their next interaction.
func (er *EventRunner) handleTaskCompleted(sessionID string, payload events.TaskCompletedPayload) {
	summary := fmt.Sprintf("[Task completed] %s", payload.Title)
	if payload.OutputSummary != "" {
		summary += "\n" + payload.OutputSummary
	}

	msg := sessions.Message{
		Role:    "system",
		Content: summary,
		Ts:      time.Now(),
	}
	if err := er.store.AppendMessage(sessionID, msg); err != nil {
		slog.Error("persist task completed notification", "error", err, "session_id", sessionID)
	}
}

// persistToolLog saves a compact summary of a completed/failed tool call.
func (er *EventRunner) persistToolLog(sessionID string, p events.ToolCallPayload) {
	if p.Status != events.ToolStatusCompleted && p.Status != events.ToolStatusFailed {
		return
	}
	summary := p.Name
	if p.Error != "" {
		summary += " ✗ " + truncate(p.Error, 100)
	} else if p.Result != "" {
		summary += " → " + truncate(p.Result, 200)
	}
	msg := sessions.Message{Role: sessions.RoleToolLog, Content: summary, Ts: time.Now()}
	if err := er.store.AppendMessage(sessionID, msg); err != nil {
		slog.Error("persist tool log", "error", err, "session_id", sessionID)
	}
}

// truncate shortens s to maxLen runes, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

// Close stops the event runner.
func (er *EventRunner) Close() {
	er.cancel()
	if er.unsubscribe != nil {
		er.unsubscribe()
	}
}

func (er *EventRunner) emitError(sessionID string, errMsg string) {
	er.bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.AssistantMessagePayload{
		Error: errMsg,
	}, sessionID))
}

func (er *EventRunner) emitResponse(sessionID string, content string) {
	er.bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.AssistantMessagePayload{
		Content: content,
	}, sessionID))
}

func (er *EventRunner) emitStreamStart(sessionID string) {
	er.bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.AssistantStreamPayload{
		Phase: events.StreamPhaseStart,
	}, sessionID))
}

func (er *EventRunner) emitStreamDelta(sessionID string, content string) {
	seq := atomic.AddInt32(&er.streamSeqIdx, 1)
	er.bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.AssistantStreamPayload{
		Phase:   events.StreamPhaseDelta,
		Content: content,
		Index:   int(seq),
	}, sessionID))
}

func (er *EventRunner) emitStreamEnd(sessionID string) {
	er.bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.AssistantStreamPayload{
		Phase: events.StreamPhaseEnd,
	}, sessionID))
}

// toolNotFoundRe matches Eino ADK errors like "tool submit_task not found in toolsNode indexes".
var toolNotFoundRe = regexp.MustCompile(`tool (\S+) not found in toolsNode`)

// extractMissingToolName returns the tool name from an Eino "tool not found" error,
// or "" if the error is unrelated.
func extractMissingToolName(errMsg string) string {
	if !strings.Contains(errMsg, "not found in toolsNode") {
		return ""
	}
	m := toolNotFoundRe.FindStringSubmatch(errMsg)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}
