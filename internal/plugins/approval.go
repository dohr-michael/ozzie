package plugins

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/events"
)

// promptToolApproval publishes a dangerous-tool approval prompt and waits for
// the user's response. It grants permissions on approval and returns an error
// on denial or context cancellation.
func promptToolApproval(
	ctx context.Context,
	bus *events.Bus,
	perms *ToolPermissions,
	sessionID string,
	needPrompt []string,
	msgFmt string,
) error {
	token := uuid.New().String()

	bus.Publish(events.NewTypedEventWithSession(events.SourcePlugin, events.PromptRequestPayload{
		Type:  events.PromptTypeSelect,
		Label: fmt.Sprintf(msgFmt, strings.Join(needPrompt, ", ")),
		Options: []events.PromptOption{
			{Value: "allow", Label: "Allow all listed tools"},
			{Value: "deny", Label: "Deny"},
		},
		Token: token,
	}, sessionID))

	ch, unsub := bus.SubscribeChan(1, events.EventPromptResponse)
	defer unsub()

	for {
		select {
		case event := <-ch:
			payload, ok := events.GetPromptResponsePayload(event)
			if !ok || payload.Token != token {
				continue
			}
			val, _ := payload.Value.(string)
			if val == "allow" {
				for _, name := range needPrompt {
					perms.AllowForSession(sessionID, name)
					bus.Publish(events.NewTypedEventWithSession(events.SourcePlugin,
						events.ToolApprovedPayload{ToolName: name}, sessionID))
				}
				return nil
			}
			return fmt.Errorf("dangerous tools denied by user: %s", strings.Join(needPrompt, ", "))
		case <-ctx.Done():
			return fmt.Errorf("waiting for tool approval: %w", ctx.Err())
		}
	}
}
