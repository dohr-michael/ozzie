package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	wsclient "github.com/dohr-michael/ozzie/clients/ws"
	"github.com/dohr-michael/ozzie/internal/events"
)

// NewAskCommand returns the ask subcommand.
func NewAskCommand() *cli.Command {
	return &cli.Command{
		Name:      "ask",
		Usage:     "Send a message to the agent and print the response",
		ArgsUsage: "<message>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "gateway",
				Usage: "Gateway WebSocket URL",
				Value: "ws://127.0.0.1:18420/api/ws",
			},
		},
		Action: runAsk,
	}
}

func runAsk(_ context.Context, cmd *cli.Command) error {
	message := cmd.Args().First()
	if message == "" {
		return fmt.Errorf("usage: ozzie ask <message>")
	}

	gatewayURL := cmd.String("gateway")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := wsclient.Dial(ctx, gatewayURL)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}
	defer client.Close()

	if err := client.SendMessage(message); err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	// Read frames until we get the final assistant.message
	streaming := false
	for {
		frame, err := client.ReadFrame()
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("timeout waiting for response")
			}
			return fmt.Errorf("read frame: %w", err)
		}

		if frame.Event == "" {
			continue
		}

		switch events.EventType(frame.Event) {
		case events.EventAssistantStream:
			var evt events.Event
			if err := json.Unmarshal(frame.Payload, &evt); err != nil {
				continue
			}
			payload, ok := events.GetAssistantStreamPayload(evt)
			if !ok {
				continue
			}

			switch payload.Phase {
			case events.StreamPhaseStart:
				streaming = true
			case events.StreamPhaseDelta:
				fmt.Fprint(os.Stdout, payload.Content)
			case events.StreamPhaseEnd:
				if streaming {
					fmt.Fprintln(os.Stdout)
				}
			}

		case events.EventAssistantMessage:
			var evt events.Event
			if err := json.Unmarshal(frame.Payload, &evt); err != nil {
				continue
			}
			payload, ok := events.GetAssistantMessagePayload(evt)
			if !ok {
				continue
			}

			if payload.Error != "" {
				return fmt.Errorf("agent error: %s", payload.Error)
			}

			if !streaming && payload.Content != "" {
				fmt.Fprintln(os.Stdout, payload.Content)
			}
			return nil
		}
	}
}
