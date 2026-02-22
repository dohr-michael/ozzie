package agent

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/sessions"
)

// EventRunner wraps an ADK Runner and provides event-driven execution.
type EventRunner struct {
	runner *adk.Runner
	bus    *events.Bus
	store  sessions.Store

	mu           sync.Mutex
	running      map[string]bool // per-session lock
	streamSeqIdx int32

	ctx         context.Context
	cancel      context.CancelFunc
	unsubscribe func()
}

// EventRunnerConfig contains configuration for the EventRunner.
type EventRunnerConfig struct {
	Runner   *adk.Runner
	EventBus *events.Bus
	Store    sessions.Store
}

// NewEventRunner creates a new event-driven runner.
func NewEventRunner(cfg EventRunnerConfig) *EventRunner {
	ctx, cancel := context.WithCancel(context.Background())

	er := &EventRunner{
		runner:  cfg.Runner,
		bus:     cfg.EventBus,
		store:   cfg.Store,
		running: make(map[string]bool),
		ctx:     ctx,
		cancel:  cancel,
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
	userMsg := sessions.Message{Role: string(schema.User), Content: content}
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

	er.emitStreamStart(sessionID)
	er.runAgent(sessionID, messages)
}

func (er *EventRunner) runAgent(sessionID string, messages []*schema.Message) {
	checkpointID := uuid.New().String()
	iter := er.runner.Run(er.ctx, messages, adk.WithCheckPointID(checkpointID))
	er.consumeIterator(sessionID, iter)
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

	// Persist assistant response
	if contentBuilder != "" {
		assistantMsg := sessions.Message{Role: string(schema.Assistant), Content: contentBuilder}
		if err := er.store.AppendMessage(sessionID, assistantMsg); err != nil {
			slog.Error("persist assistant message", "error", err, "session_id", sessionID)
		}
	}

	er.emitResponse(sessionID, contentBuilder)
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
