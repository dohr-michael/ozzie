// Package callbacks provides Eino callback handlers that bridge to the event bus.
package callbacks

import (
	"context"
	"io"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	ub "github.com/cloudwego/eino/utils/callbacks"

	"github.com/dohr-michael/ozzie/internal/events"
)

// NewEventBusHandler creates a callback handler that publishes events to the bus.
func NewEventBusHandler(bus *events.Bus, source events.EventSource) callbacks.Handler {
	if source == "" {
		source = events.SourceAgent
	}

	publishTyped := func(ctx context.Context, payload events.EventPayload) {
		if sid := events.SessionIDFromContext(ctx); sid != "" {
			bus.Publish(events.NewTypedEventWithSession(source, payload, sid))
		} else {
			bus.Publish(events.NewTypedEvent(source, payload))
		}
	}

	modelHandler := &ub.ModelCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *model.CallbackInput) context.Context {
			publishTyped(ctx, events.LLMCallPayload{
				Phase:        "request",
				Model:        info.Name,
				MessageCount: len(input.Messages),
			})
			return ctx
		},

		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
			payload := events.LLMCallPayload{
				Phase: "response",
				Model: info.Name,
			}
			if output.Message != nil && output.Message.ResponseMeta != nil && output.Message.ResponseMeta.Usage != nil {
				payload.TokensInput = output.Message.ResponseMeta.Usage.PromptTokens
				payload.TokensOutput = output.Message.ResponseMeta.Usage.CompletionTokens
			}
			publishTyped(ctx, payload)
			return ctx
		},

		OnEndWithStreamOutput: func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
			// Stream is a copy — must be drained. Run in goroutine to avoid blocking.
			go func() {
				defer output.Close()
				var tokensIn, tokensOut int
				for {
					chunk, err := output.Recv()
					if err != nil {
						if err != io.EOF {
							publishTyped(ctx, events.LLMCallPayload{
								Phase: "error",
								Model: info.Name,
								Error: err.Error(),
							})
						}
						break
					}
					// Prefer TokenUsage on CallbackOutput (set by provider libs)
					if chunk.TokenUsage != nil {
						if chunk.TokenUsage.PromptTokens > 0 {
							tokensIn = chunk.TokenUsage.PromptTokens
						}
						if chunk.TokenUsage.CompletionTokens > 0 {
							tokensOut = chunk.TokenUsage.CompletionTokens
						}
					}
					// Fallback: check Message.ResponseMeta.Usage
					if chunk.Message != nil && chunk.Message.ResponseMeta != nil && chunk.Message.ResponseMeta.Usage != nil {
						u := chunk.Message.ResponseMeta.Usage
						if u.PromptTokens > 0 {
							tokensIn = u.PromptTokens
						}
						if u.CompletionTokens > 0 {
							tokensOut = u.CompletionTokens
						}
					}
				}
				publishTyped(ctx, events.LLMCallPayload{
					Phase:        "response",
					Model:        info.Name,
					TokensInput:  tokensIn,
					TokensOutput: tokensOut,
				})
			}()
			return ctx
		},

		OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			publishTyped(ctx, events.LLMCallPayload{
				Phase: "error",
				Model: info.Name,
				Error: err.Error(),
			})
			return ctx
		},
	}

	toolHandler := &ub.ToolCallbackHandler{
		OnStart: func(ctx context.Context, info *callbacks.RunInfo, input *tool.CallbackInput) context.Context {
			payload := events.ToolCallPayload{
				Status: events.ToolStatusStarted,
				Name:   info.Name,
			}
			if input.ArgumentsInJSON != "" {
				payload.Arguments = map[string]any{"raw": truncatePayload(input.ArgumentsInJSON, 1000)}
			}
			publishTyped(ctx, payload)
			return ctx
		},
		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *tool.CallbackOutput) context.Context {
			payload := events.ToolCallPayload{
				Status: events.ToolStatusCompleted,
				Name:   info.Name,
				Result: truncatePayload(output.Response, 1000),
			}
			publishTyped(ctx, payload)
			return ctx
		},
		OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			publishTyped(ctx, events.ToolCallPayload{
				Status: events.ToolStatusFailed,
				Name:   info.Name,
				Error:  err.Error(),
			})
			return ctx
		},
	}

	return ub.NewHandlerHelper().
		ChatModel(modelHandler).
		Tool(toolHandler).
		Handler()
}

func truncatePayload(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}
