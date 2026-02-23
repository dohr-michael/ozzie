package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/events"
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

// TaskStore provides mailbox access for the EventRunner.
type TaskStore interface {
	LoadMailbox(taskID string) ([]TaskMailboxMessage, error)
}

// TaskMailboxMessage mirrors tasks.MailboxMessage to avoid import cycle.
type TaskMailboxMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// EventRunner wraps an AgentFactory and provides event-driven execution
// with dynamic tool selection.
type EventRunner struct {
	factory    *AgentFactory
	toolSet    *ToolSet
	registry   ToolLookup
	bus        *events.Bus
	store      sessions.Store
	taskStore  TaskStore
	compressor *Compressor

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
	TaskStore       TaskStore    // task store for mailbox access (optional)
	Pool            CapacityPool // actor pool for capacity management (optional)
	DefaultProvider string       // default provider name for AcquireInteractive
	ContextWindow   int          // total context window in tokens (for compression)
}

// NewEventRunner creates a new event-driven runner.
func NewEventRunner(cfg EventRunnerConfig) *EventRunner {
	ctx, cancel := context.WithCancel(context.Background())

	er := &EventRunner{
		factory:         cfg.Factory,
		toolSet:         cfg.ToolSet,
		registry:        cfg.Registry,
		bus:             cfg.EventBus,
		store:           cfg.Store,
		taskStore:       cfg.TaskStore,
		compressor:      NewCompressor(CompressorConfig{ContextWindow: cfg.ContextWindow}),
		pool:            cfg.Pool,
		defaultProvider: cfg.DefaultProvider,
		running:         make(map[string]bool),
		ctx:             ctx,
		cancel:          cancel,
	}

	er.unsubscribe = cfg.EventBus.Subscribe(er.handleEvent,
		events.EventUserMessage,
		events.EventTaskCompleted,
		events.EventTaskSuspended,
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
	case events.EventTaskSuspended:
		if payload, ok := events.GetTaskSuspendedPayload(event); ok && event.SessionID != "" {
			go er.handleTaskSuspended(event.SessionID, payload)
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
		msg := m.ToSchemaMessage()
		// Skip messages with empty content — APIs reject these.
		if msg.Content == "" && msg.Role != schema.Assistant {
			continue
		}
		messages = append(messages, msg)
	}

	// Context compression — estimate system prompt as persona only;
	// dynamic context (tools, session, memories) is injected by middlewares.
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

	// Retry loop: up to 2 attempts (initial + retry after tool activation)
	for attempt := 0; attempt < 2; attempt++ {
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

			content := er.runAgentBuffered(sessionID, runner, messages)

			if er.toolSet.ActivatedDuringTurn(sessionID) {
				// Tools were activated — retry with expanded tool set (streamed)
				slog.Info("tools activated, retrying with expanded set",
					"session_id", sessionID)
				continue
			}

			// No activation — emit the buffered response
			er.emitStreamStart(sessionID)
			if content != "" {
				er.emitStreamDelta(sessionID, content)
				er.emitStreamEnd(sessionID)
			}
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
	checkpointID := uuid.New().String()
	iter := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))
	er.consumeIterator(sessionID, iter)
}

func (er *EventRunner) runAgentBuffered(sessionID string, runner *adk.Runner, messages []*schema.Message) string {
	ctx := events.ContextWithSessionID(er.ctx, sessionID)
	checkpointID := uuid.New().String()
	iter := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))
	return er.consumeIteratorBuffered(sessionID, iter)
}

func (er *EventRunner) consumeIterator(sessionID string, iter *adk.AsyncIterator[*adk.AgentEvent]) {
	var contentBuilder string

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			slog.Error("agent error", "error", event.Err)
			er.emitError(sessionID, event.Err.Error())
			return
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		mv := event.Output.MessageOutput

		// Tool results (intermediate ReAct steps) — skip, callbacks handle emission
		if mv.Role == schema.Tool {
			// Consume the stream to avoid leaking goroutines
			if mv.IsStreaming && mv.MessageStream != nil {
				mv.MessageStream.Close()
			}
			continue
		}

		// Assistant message — may contain tool calls (intermediate) or final text
		if mv.IsStreaming {
			content := er.consumeStream(sessionID, mv.MessageStream)
			if content != "" {
				contentBuilder = content
				er.emitStreamEnd(sessionID)
			}
		} else if mv.Message != nil {
			// Intermediate assistant messages with tool calls — skip accumulation
			if len(mv.Message.ToolCalls) > 0 && mv.Message.Content == "" {
				continue
			}
			if mv.Message.Content != "" {
				contentBuilder = mv.Message.Content
				er.emitStreamDelta(sessionID, contentBuilder)
				er.emitStreamEnd(sessionID)
			}
		}
	}

	er.persistAndEmitResponse(sessionID, contentBuilder)
}

func (er *EventRunner) consumeIteratorBuffered(sessionID string, iter *adk.AsyncIterator[*adk.AgentEvent]) string {
	var contentBuilder string

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			slog.Error("agent error (buffered)", "error", event.Err)
			er.emitError(sessionID, event.Err.Error())
			return ""
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		mv := event.Output.MessageOutput

		// Tool results — consume streams to avoid leaks
		if mv.Role == schema.Tool {
			if mv.IsStreaming && mv.MessageStream != nil {
				mv.MessageStream.Close()
			}
			continue
		}

		// Assistant message — collect content without streaming
		if mv.IsStreaming {
			content := er.consumeStreamBuffered(mv.MessageStream)
			if content != "" {
				contentBuilder = content
			}
		} else if mv.Message != nil {
			if len(mv.Message.ToolCalls) > 0 && mv.Message.Content == "" {
				continue
			}
			if mv.Message.Content != "" {
				contentBuilder = mv.Message.Content
			}
		}
	}

	return contentBuilder
}

func (er *EventRunner) consumeStream(sessionID string, stream *schema.StreamReader[*schema.Message]) string {
	var fullContent string

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			er.emitError(sessionID, err.Error())
			break
		}

		if chunk != nil && chunk.Content != "" {
			er.emitStreamDelta(sessionID, chunk.Content)
			fullContent += chunk.Content
		}
	}

	return fullContent
}

func (er *EventRunner) consumeStreamBuffered(stream *schema.StreamReader[*schema.Message]) string {
	var fullContent string

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Error("stream error (buffered)", "error", err)
			break
		}

		if chunk != nil && chunk.Content != "" {
			fullContent += chunk.Content
		}
	}

	return fullContent
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

	var content string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		mv := event.Output.MessageOutput
		if mv.IsStreaming {
			content = er.consumeStreamBuffered(mv.MessageStream)
		} else if mv.Message != nil && mv.Message.Content != "" {
			content = mv.Message.Content
		}
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

// handleTaskSuspended injects a validation request into the session so the user
// can see the plan and reply.
func (er *EventRunner) handleTaskSuspended(sessionID string, payload events.TaskSuspendedPayload) {
	// Only relay validation requests (tasks waiting for user reply)
	if payload.PlanContent == "" {
		return
	}

	summary := fmt.Sprintf("[Validation needed — %s (task %s)]\n\n%s\n\nReply with your feedback to resume this task.",
		payload.Title, payload.TaskID, payload.PlanContent)

	msg := sessions.Message{
		Role:    "system",
		Content: summary,
		Ts:      time.Now(),
	}
	if err := er.store.AppendMessage(sessionID, msg); err != nil {
		slog.Error("persist task suspended notification", "error", err, "session_id", sessionID)
	}
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
