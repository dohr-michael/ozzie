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

type modelNameKeyType struct{}

var modelNameKey = modelNameKeyType{}

// resolveModelName returns the actual model name from Config if available, falling back to info.Name.
func resolveModelName(info *callbacks.RunInfo, cfg *model.Config) string {
	if cfg != nil && cfg.Model != "" {
		return cfg.Model
	}
	return info.Name
}

func modelNameFromContext(ctx context.Context) string {
	if name, ok := ctx.Value(modelNameKey).(string); ok && name != "" {
		return name
	}
	return ""
}

// NewEventBusHandler creates a callback handler that publishes events to the bus.
func NewEventBusHandler(bus events.EventBus, source events.EventSource) callbacks.Handler {
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
			modelName := resolveModelName(info, input.Config)
			publishTyped(ctx, events.LLMCallPayload{
				Phase:        "request",
				Model:        modelName,
				MessageCount: len(input.Messages),
			})
			return context.WithValue(ctx, modelNameKey, modelName)
		},

		OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
			modelName := modelNameFromContext(ctx)
			if modelName == "" {
				modelName = info.Name
			}
			payload := events.LLMCallPayload{
				Phase: "response",
				Model: modelName,
			}
			if output.Message != nil && output.Message.ResponseMeta != nil && output.Message.ResponseMeta.Usage != nil {
				u := output.Message.ResponseMeta.Usage
				payload.TokensInput = u.PromptTokens
				payload.TokensOutput = u.CompletionTokens
				payload.TokensReasoning = u.CompletionTokensDetails.ReasoningTokens
			}
			publishTyped(ctx, payload)
			return ctx
		},

		OnEndWithStreamOutput: func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
			modelName := modelNameFromContext(ctx)
			if modelName == "" {
				modelName = info.Name
			}
			// Stream is a copy — must be drained. Run in goroutine to avoid blocking.
			go func() {
				defer output.Close()
				var tokensIn, tokensOut, tokensReasoning int
				for {
					chunk, err := output.Recv()
					if err != nil {
						if err != io.EOF {
							publishTyped(ctx, events.LLMCallPayload{
								Phase: "error",
								Model: modelName,
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
						if chunk.TokenUsage.CompletionTokensDetails.ReasoningTokens > 0 {
							tokensReasoning = chunk.TokenUsage.CompletionTokensDetails.ReasoningTokens
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
						if u.CompletionTokensDetails.ReasoningTokens > 0 {
							tokensReasoning = u.CompletionTokensDetails.ReasoningTokens
						}
					}
				}
				publishTyped(ctx, events.LLMCallPayload{
					Phase:           "response",
					Model:           modelName,
					TokensInput:     tokensIn,
					TokensOutput:    tokensOut,
					TokensReasoning: tokensReasoning,
				})
			}()
			return ctx
		},

		OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			modelName := modelNameFromContext(ctx)
			if modelName == "" {
				modelName = info.Name
			}
			publishTyped(ctx, events.LLMCallPayload{
				Phase: "error",
				Model: modelName,
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
	if maxLen <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "... (truncated)"
}
