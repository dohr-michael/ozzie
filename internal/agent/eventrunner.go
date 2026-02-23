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

// EventRunner wraps an AgentFactory and provides event-driven execution
// with dynamic tool selection.
type EventRunner struct {
	factory    *AgentFactory
	toolSet    *ToolSet
	registry   ToolLookup
	bus        *events.Bus
	store      sessions.Store
	compressor *Compressor

	composer            *PromptComposer
	customInstructions  string
	allToolDescriptions map[string]string
	skillDescriptions   map[string]string

	mu           sync.Mutex
	running      map[string]bool // per-session lock
	streamSeqIdx int32

	ctx         context.Context
	cancel      context.CancelFunc
	unsubscribe func()
}

// EventRunnerConfig contains configuration for the EventRunner.
type EventRunnerConfig struct {
	Factory             *AgentFactory
	ToolSet             *ToolSet
	Registry            ToolLookup
	EventBus            *events.Bus
	Store               sessions.Store
	ContextWindow       int               // total context window in tokens
	CustomInstructions  string            // Layer 2: from config.Agent.SystemPrompt
	AllToolDescriptions map[string]string // Full catalog: tool name → description
	SkillDescriptions   map[string]string // Layer 3b: skill name → description
}

// NewEventRunner creates a new event-driven runner.
func NewEventRunner(cfg EventRunnerConfig) *EventRunner {
	ctx, cancel := context.WithCancel(context.Background())

	er := &EventRunner{
		factory:             cfg.Factory,
		toolSet:             cfg.ToolSet,
		registry:            cfg.Registry,
		bus:                 cfg.EventBus,
		store:               cfg.Store,
		compressor:          NewCompressor(CompressorConfig{ContextWindow: cfg.ContextWindow}),
		composer:            NewPromptComposer(),
		customInstructions:  cfg.CustomInstructions,
		allToolDescriptions: cfg.AllToolDescriptions,
		skillDescriptions:   cfg.SkillDescriptions,
		running:             make(map[string]bool),
		ctx:                 ctx,
		cancel:              cancel,
	}

	er.unsubscribe = cfg.EventBus.Subscribe(er.handleEvent,
		events.EventUserMessage,
	)

	return er
}

func (er *EventRunner) handleEvent(event events.Event) {
	switch event.Type {
	case events.EventUserMessage:
		if payload, ok := events.GetUserMessagePayload(event); ok && payload.Content != "" {
			go er.processMessage(event.SessionID, payload.Content)
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

	messages := make([]*schema.Message, len(history))
	for i, m := range history {
		messages[i] = m.ToSchemaMessage()
	}

	// Context compression
	if session, err := er.store.Get(sessionID); err == nil {
		sysEstimate := er.compressor.EstimateTokens([]*schema.Message{{
			Role:    schema.System,
			Content: er.estimateSystemPrompt(sessionID, len(history)),
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

		fullMessages := er.prependDynamicContext(sessionID, messages, len(history), activeNames)

		// First attempt with inactive tools: run buffered (non-streaming) to detect activation
		if attempt == 0 && er.toolSet.HasInactiveTools(sessionID) {
			runner, err := er.factory.CreateRunnerBuffered(er.ctx, tools)
			if err != nil {
				slog.Error("create runner (buffered)", "error", err, "session_id", sessionID)
				er.emitError(sessionID, "failed to create agent runner")
				return
			}

			content := er.runAgentBuffered(sessionID, runner, fullMessages)

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
		er.runAgent(sessionID, runner, fullMessages)
		return
	}
}

func (er *EventRunner) prependDynamicContext(sessionID string, messages []*schema.Message, msgCount int, activeNames []string) []*schema.Message {
	pctx := PromptContext{
		CustomInstructions:  er.customInstructions,
		ActiveToolNames:     activeNames,
		AllToolDescriptions: er.allToolDescriptions,
		SkillDescriptions:   er.skillDescriptions,
		MessageCount:        msgCount,
	}

	// Load session metadata (ignore errors — session may not exist yet)
	if sess, err := er.store.Get(sessionID); err == nil {
		pctx.Session = sess
	}

	dynamic := er.composer.Compose(pctx)
	if dynamic == "" {
		return messages
	}

	systemMsg := &schema.Message{Role: schema.System, Content: dynamic}
	return append([]*schema.Message{systemMsg}, messages...)
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

// estimateSystemPrompt generates the system prompt text for token estimation.
func (er *EventRunner) estimateSystemPrompt(sessionID string, msgCount int) string {
	pctx := PromptContext{
		CustomInstructions:  er.customInstructions,
		AllToolDescriptions: er.allToolDescriptions,
		SkillDescriptions:   er.skillDescriptions,
		MessageCount:        msgCount,
	}
	if sess, err := er.store.Get(sessionID); err == nil {
		pctx.Session = sess
	}

	return er.factory.Persona() + "\n\n" + er.composer.Compose(pctx)
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
