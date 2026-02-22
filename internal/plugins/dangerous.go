package plugins

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/events"
)

const confirmationTimeout = 60 * time.Second

// DangerousToolWrapper wraps a tool.InvokableTool with a confirmation step.
// Before executing, it emits a confirmation event and waits for the user response.
type DangerousToolWrapper struct {
	inner tool.InvokableTool
	name  string
	bus   *events.Bus
}

// WrapDangerous wraps a tool with confirmation if dangerous is true.
func WrapDangerous(t tool.InvokableTool, name string, dangerous bool, bus *events.Bus) tool.InvokableTool {
	if !dangerous {
		return t
	}
	return &DangerousToolWrapper{
		inner: t,
		name:  name,
		bus:   bus,
	}
}

// Info delegates to the inner tool.
func (d *DangerousToolWrapper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return d.inner.Info(ctx)
}

// InvokableRun asks for confirmation before executing the tool.
func (d *DangerousToolWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	token := uuid.New().String()

	// Emit confirmation request
	d.bus.Publish(events.NewTypedEvent(events.SourcePlugin, events.ToolCallPayload{
		Status:    events.ToolStatusStarted,
		Name:      d.name,
		Arguments: map[string]any{"raw": argumentsInJSON},
	}))

	d.bus.Publish(events.NewTypedEvent(events.SourcePlugin, events.PromptRequestPayload{
		Type:  events.PromptTypeConfirm,
		Label: fmt.Sprintf("Allow %q to execute? Arguments: %s", d.name, truncate(argumentsInJSON, 200)),
		Token: token,
	}))

	// Wait for response
	ch, unsub := d.bus.SubscribeChan(1, events.EventPromptResponse)
	defer unsub()

	ctx, cancel := context.WithTimeout(ctx, confirmationTimeout)
	defer cancel()

	for {
		select {
		case event := <-ch:
			payload, ok := events.GetPromptResponsePayload(event)
			if !ok || payload.Token != token {
				continue
			}
			if payload.Cancelled {
				return "", fmt.Errorf("tool %q execution denied by user", d.name)
			}
			// Confirmed — execute
			return d.inner.InvokableRun(ctx, argumentsInJSON, opts...)
		case <-ctx.Done():
			return "", fmt.Errorf("tool %q confirmation timed out", d.name)
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

var _ tool.InvokableTool = (*DangerousToolWrapper)(nil)
