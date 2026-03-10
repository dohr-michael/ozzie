package plugins

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/events"
)

// DangerousToolWrapper wraps a tool.InvokableTool with a confirmation step.
// Before executing, it emits a prompt event and waits for the user response.
// If the tool is pre-approved via ToolPermissions, execution proceeds immediately.
// In sub-tasks, the prompt is routed to the parent session via session ID.
type DangerousToolWrapper struct {
	inner tool.InvokableTool
	name  string
	bus   events.EventBus
	perms *ToolPermissions
}

// WrapDangerous wraps a tool with confirmation if dangerous is true.
func WrapDangerous(t tool.InvokableTool, name string, dangerous bool, bus events.EventBus, perms *ToolPermissions) tool.InvokableTool {
	if !dangerous {
		return t
	}
	return &DangerousToolWrapper{
		inner: t,
		name:  name,
		bus:   bus,
		perms: perms,
	}
}

// Info delegates to the inner tool.
func (d *DangerousToolWrapper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return d.inner.Info(ctx)
}

// InvokableRun checks permissions before executing the tool.
// If pre-approved (global or session), executes immediately.
// Otherwise, prompts the user with three options: allow once, always allow, deny.
// The prompt is routed via session ID so sub-tasks bubble up to the parent client.
// There is no timeout — waits until the user responds or the context is cancelled.
func (d *DangerousToolWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	sessionID := events.SessionIDFromContext(ctx)

	// Check permissions — pre-approved tools execute immediately
	if d.perms != nil && d.perms.IsAllowed(sessionID, d.name) {
		return d.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	}

	// Prompt the user — works for both interactive and sub-task contexts
	// because the session ID routes the event to the correct client.
	token := uuid.New().String()

	d.bus.Publish(events.NewTypedEventWithSession(events.SourcePlugin, events.ToolCallPayload{
		Status:    events.ToolStatusStarted,
		Name:      d.name,
		Arguments: map[string]any{"raw": argumentsInJSON},
	}, sessionID))

	d.bus.Publish(events.NewTypedEventWithSession(events.SourcePlugin, events.PromptRequestPayload{
		Type:  events.PromptTypeSelect,
		Label: fmt.Sprintf("Tool %q requires approval. Arguments: %s", d.name, truncate(argumentsInJSON, 200)),
		Options: []events.PromptOption{
			{Value: "once", Label: "Allow once"},
			{Value: "session", Label: "Always allow for this session"},
			{Value: "deny", Label: "Deny"},
		},
		Token: token,
	}, sessionID))

	ch, unsub := d.bus.SubscribeChan(1, events.EventPromptResponse)
	defer unsub()

	for {
		select {
		case event := <-ch:
			payload, ok := events.GetPromptResponsePayload(event)
			if !ok || payload.Token != token {
				continue
			}
			switch val := payload.Value.(type) {
			case string:
				switch val {
				case "once":
					return d.inner.InvokableRun(ctx, argumentsInJSON, opts...)
				case "session":
					if d.perms != nil && sessionID != "" {
						d.perms.AllowForSession(sessionID, d.name)
					}
					d.bus.Publish(events.NewTypedEventWithSession(events.SourcePlugin,
						events.ToolApprovedPayload{ToolName: d.name}, sessionID))
					return d.inner.InvokableRun(ctx, argumentsInJSON, opts...)
				default:
					return "", fmt.Errorf("tool %q execution denied by user", d.name)
				}
			default:
				// Cancelled or unknown value type
				if payload.Cancelled {
					return "", fmt.Errorf("tool %q execution denied by user", d.name)
				}
				return "", fmt.Errorf("tool %q execution denied by user", d.name)
			}
		case <-ctx.Done():
			return "", fmt.Errorf("tool %q: waiting for approval: %w", d.name, ctx.Err())
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
