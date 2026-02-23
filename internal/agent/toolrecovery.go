package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/cloudwego/eino/compose"
)

// DefaultMaxToolRetries is the maximum number of times a tool error will be
// converted into a textual result before the error is propagated. The counter
// is tracked per tool name so that different tools have independent budgets.
const DefaultMaxToolRetries = 3

// ToolRecoveryConfig configures the tool-call error recovery middleware.
type ToolRecoveryConfig struct {
	// MaxRetries is the number of recoverable errors per tool name.
	// Zero means DefaultMaxToolRetries.
	MaxRetries int
}

// NewToolRecoveryMiddleware returns an Eino ToolMiddleware that intercepts tool
// errors and converts them into textual results so the LLM can adjust its
// parameters and retry. After MaxRetries consecutive failures for the same tool
// name the original error is propagated, stopping the agent loop.
func NewToolRecoveryMiddleware(cfg ToolRecoveryConfig) compose.ToolMiddleware {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = DefaultMaxToolRetries
	}

	var mu sync.Mutex
	counts := make(map[string]int)

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

				mu.Lock()
				counts[input.Name]++
				count := counts[input.Name]
				mu.Unlock()

				if count >= maxRetries {
					slog.Error("tool error recovery: max retries reached, propagating error",
						"tool", input.Name,
						"attempt", count,
						"max", maxRetries,
						"error", err,
					)
					return nil, err
				}

				msg := formatToolError(input.Name, count, maxRetries, err)
				slog.Warn("tool error recovery: converting error to result",
					"tool", input.Name,
					"attempt", count,
					"max", maxRetries,
					"error", err,
				)
				return &compose.ToolOutput{Result: msg}, nil
			}
		},
	}
}

// formatToolError builds the textual error message sent back to the LLM.
func formatToolError(toolName string, attempt, maxRetries int, err error) string {
	return fmt.Sprintf(
		`[TOOL_ERROR] Tool %q failed (attempt %d/%d): %s
You can retry with different parameters, or inform the user about the issue.`,
		toolName, attempt, maxRetries, err,
	)
}
