package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/compose"
)

// ToolRecoveryConfig configures the tool-call error recovery middleware.
// Currently a marker type — all errors are converted to textual results
// unconditionally. The runner's maxIterations (20) guards against infinite
// retry loops.
type ToolRecoveryConfig struct{}

// NewToolRecoveryMiddleware returns an Eino ToolMiddleware that intercepts tool
// errors and converts them into textual results so the LLM can decide whether to
// retry with different parameters or inform the user.
// Errors are never propagated — they are always returned as text to avoid
// crashing the session via event.Err in consumeIterator.
func NewToolRecoveryMiddleware(_ ToolRecoveryConfig) compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				out, err := next(ctx, input)
				if err == nil {
					// Guard against empty tool results — OpenAI/Ollama APIs
					// reject tool_result messages with empty content.
					if out != nil && out.Result == "" {
						out.Result = "[OK]"
					}
					return out, nil
				}

				slog.Warn("tool error recovery: converting error to result",
					"tool", input.Name, "error", err)
				msg := fmt.Sprintf(
					"[TOOL_ERROR] Tool %q failed: %s\n"+
						"You can retry with different parameters, or inform the user about the issue.",
					input.Name, err)
				return &compose.ToolOutput{Result: msg}, nil
			}
		},
	}
}
