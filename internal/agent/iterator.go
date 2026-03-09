package agent

import (
	"io"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// IterCallbacks configures optional behavior for ConsumeIterator.
type IterCallbacks struct {
	// ShouldPreempt returns true when the caller wants to abort iteration early.
	// Checked between ReAct iterations. When triggered, ConsumeIterator returns
	// the content accumulated so far and ErrIterPreempted.
	ShouldPreempt func() bool

	// OnStreamChunk is called for each streamed text chunk (e.g. to emit deltas).
	OnStreamChunk func(string)

	// OnStreamDone is called when a streaming message is fully consumed.
	OnStreamDone func()

	// OnError is called on agent errors. If nil, the error is returned directly.
	OnError func(error)
}

// ErrIterPreempted is returned when ShouldPreempt fires during iteration.
var ErrIterPreempted = errIterPreempted{}

type errIterPreempted struct{}

func (errIterPreempted) Error() string { return "iterator preempted" }

// ConsumeIterator drains an ADK AsyncIterator and returns the last accumulated
// text content. It handles tool messages, streaming, and non-streaming outputs.
func ConsumeIterator(iter *adk.AsyncIterator[*adk.AgentEvent], cb IterCallbacks) (string, error) {
	var content string

	for {
		if cb.ShouldPreempt != nil && cb.ShouldPreempt() {
			return content, ErrIterPreempted
		}

		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			if cb.OnError != nil {
				cb.OnError(event.Err)
				return content, event.Err
			}
			return content, event.Err
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		mv := event.Output.MessageOutput

		// Tool results (intermediate ReAct steps) — consume streams to avoid leaks.
		if mv.Role == schema.Tool {
			if mv.IsStreaming && mv.MessageStream != nil {
				mv.MessageStream.Close()
			}
			continue
		}

		// Assistant message — streaming or buffered.
		if mv.IsStreaming && mv.MessageStream != nil {
			text := consumeStreamGeneric(mv.MessageStream, cb.OnStreamChunk)
			if text != "" {
				content = text
				if cb.OnStreamDone != nil {
					cb.OnStreamDone()
				}
			}
		} else if mv.Message != nil {
			// Skip tool-call-only messages (no text content).
			if len(mv.Message.ToolCalls) > 0 && mv.Message.Content == "" {
				continue
			}
			if mv.Message.Content != "" {
				content = mv.Message.Content
				if cb.OnStreamChunk != nil {
					cb.OnStreamChunk(content)
				}
				if cb.OnStreamDone != nil {
					cb.OnStreamDone()
				}
			}
		}
	}

	return content, nil
}

// consumeStreamGeneric reads all chunks from a streaming message.
// If onChunk is non-nil, each chunk is forwarded to it.
func consumeStreamGeneric(stream *schema.StreamReader[*schema.Message], onChunk func(string)) string {
	var sb strings.Builder

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if chunk != nil && chunk.Content != "" {
			sb.WriteString(chunk.Content)
			if onChunk != nil {
				onChunk(chunk.Content)
			}
		}
	}

	return sb.String()
}
