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
// If the tool is pre-approved via ToolPermissions, execution proceeds immediately.
type DangerousToolWrapper struct {
	inner tool.InvokableTool
	name  string
	bus   *events.Bus
	perms *ToolPermissions
}

// WrapDangerous wraps a tool with confirmation if dangerous is true.
func WrapDangerous(t tool.InvokableTool, name string, dangerous bool, bus *events.Bus, perms *ToolPermissions) tool.InvokableTool {
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
// In autonomous mode (async tasks), returns an error for unapproved tools.
// In interactive mode, prompts the user and memorizes the approval.
func (d *DangerousToolWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	sessionID := events.SessionIDFromContext(ctx)

	// Check permissions — pre-approved tools execute immediately
	if d.perms != nil && d.perms.IsAllowed(sessionID, d.name) {
		return d.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	}

	// Autonomous mode (async task) — no one to confirm, fail immediately
	if events.IsAutonomousContext(ctx) {
		return "", fmt.Errorf("tool %q requires approval but is not in allowed_dangerous list (autonomous mode)", d.name)
	}

	// Interactive mode — prompt the user
	token := uuid.New().String()

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
			// Confirmed — memorize for session and execute
			if d.perms != nil && sessionID != "" {
				d.perms.AllowForSession(sessionID, d.name)
			}
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
