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
)

// EventRunner wraps an ADK Runner and provides event-driven execution.
type EventRunner struct {
	runner   *adk.Runner
	bus      *events.Bus
	messages []*schema.Message

	mu           sync.Mutex
	running      bool
	streamSeqIdx int32

	ctx         context.Context
	cancel      context.CancelFunc
	unsubscribe func()
}

// EventRunnerConfig contains configuration for the EventRunner.
type EventRunnerConfig struct {
	Runner   *adk.Runner
	EventBus *events.Bus
}

// NewEventRunner creates a new event-driven runner.
func NewEventRunner(cfg EventRunnerConfig) *EventRunner {
	ctx, cancel := context.WithCancel(context.Background())

	er := &EventRunner{
		runner:   cfg.Runner,
		bus:      cfg.EventBus,
		messages: make([]*schema.Message, 0),
		ctx:      ctx,
		cancel:   cancel,
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
			go er.processMessage(payload.Content)
		}
	}
}

func (er *EventRunner) processMessage(content string) {
	er.mu.Lock()
	if er.running {
		er.mu.Unlock()
		return
	}
	er.running = true
	atomic.StoreInt32(&er.streamSeqIdx, 0)
	er.mu.Unlock()

	defer func() {
		er.mu.Lock()
		er.running = false
		er.mu.Unlock()
	}()

	er.messages = append(er.messages, &schema.Message{
		Role:    schema.User,
		Content: content,
	})

	er.emitStreamStart()
	er.runAgent()
}

func (er *EventRunner) runAgent() {
	checkpointID := uuid.New().String()
	iter := er.runner.Run(er.ctx, er.messages, adk.WithCheckPointID(checkpointID))
	er.consumeIterator(iter)
}

func (er *EventRunner) consumeIterator(iter *adk.AsyncIterator[*adk.AgentEvent]) {
	var contentBuilder string

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			slog.Error("agent error", "error", event.Err)
			er.emitError(event.Err.Error())
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
			content := er.consumeStream(mv.MessageStream)
			if content != "" {
				contentBuilder = content
				er.emitStreamEnd()
			}
		} else if mv.Message != nil {
			// Intermediate assistant messages with tool calls — skip accumulation
			if len(mv.Message.ToolCalls) > 0 && mv.Message.Content == "" {
				continue
			}
			if mv.Message.Content != "" {
				contentBuilder = mv.Message.Content
				er.emitStreamDelta(contentBuilder)
				er.emitStreamEnd()
			}
		}
	}

	if contentBuilder != "" {
		er.messages = append(er.messages, &schema.Message{
			Role:    schema.Assistant,
			Content: contentBuilder,
		})
	}

	er.emitResponse(contentBuilder)
}

func (er *EventRunner) consumeStream(stream *schema.StreamReader[*schema.Message]) string {
	var fullContent string

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			er.emitError(err.Error())
			break
		}

		if chunk != nil && chunk.Content != "" {
			er.emitStreamDelta(chunk.Content)
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

func (er *EventRunner) emitError(errMsg string) {
	er.bus.Publish(events.NewTypedEvent("agent", events.AssistantMessagePayload{
		Error: errMsg,
	}))
}

func (er *EventRunner) emitResponse(content string) {
	er.bus.Publish(events.NewTypedEvent("agent", events.AssistantMessagePayload{
		Content: content,
	}))
}

func (er *EventRunner) emitStreamStart() {
	er.bus.Publish(events.NewTypedEvent("agent", events.AssistantStreamPayload{
		Phase: events.StreamPhaseStart,
	}))
}

func (er *EventRunner) emitStreamDelta(content string) {
	seq := atomic.AddInt32(&er.streamSeqIdx, 1)
	er.bus.Publish(events.NewTypedEvent("agent", events.AssistantStreamPayload{
		Phase:   events.StreamPhaseDelta,
		Content: content,
		Index:   int(seq),
	}))
}

func (er *EventRunner) emitStreamEnd() {
	er.bus.Publish(events.NewTypedEvent("agent", events.AssistantStreamPayload{
		Phase: events.StreamPhaseEnd,
	}))
}
