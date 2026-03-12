package eyes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dohr-michael/ozzie/internal/core/events"
)

// AdminSender can send messages to an admin channel.
type AdminSender interface {
	AdminChannel() string
	SendAdmin(ctx context.Context, content string) error
}

// PairingNotifier subscribes to pairing request events and forwards
// formatted notifications to the Discord admin channel.
type PairingNotifier struct {
	sender AdminSender
	unsub  func()
}

// NewPairingNotifier creates a notifier that sends pairing requests to the admin channel.
func NewPairingNotifier(bus events.EventBus, sender AdminSender) *PairingNotifier {
	pn := &PairingNotifier{sender: sender}

	pn.unsub = bus.Subscribe(func(e events.Event) {
		payload, ok := events.GetPairingRequestPayload(e)
		if !ok {
			return
		}
		pn.notify(payload)
	}, events.EventPairingRequest)

	return pn
}

// Stop unsubscribes from events.
func (pn *PairingNotifier) Stop() {
	if pn.unsub != nil {
		pn.unsub()
	}
}

func (pn *PairingNotifier) notify(p events.PairingRequestPayload) {
	if pn.sender.AdminChannel() == "" {
		slog.Warn("pairing request received but no admin channel configured",
			"platform", p.Platform, "user", p.UserName)
		return
	}

	msg := fmt.Sprintf("**Pairing Request**\n"+
		"Platform: `%s`\n"+
		"User: `%s` (ID: `%s`)\n"+
		"Server: `%s` | Channel: `%s`\n"+
		"Message: %s\n\n"+
		"Use `approve_pairing` tool to grant access.",
		p.Platform, p.UserName, p.UserID,
		p.ServerID, p.ChannelID,
		p.Content,
	)

	if err := pn.sender.SendAdmin(context.Background(), msg); err != nil {
		slog.Error("failed to send pairing notification", "error", err)
	}
}
