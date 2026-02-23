package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
			&cli.StringFlag{
				Name:    "session",
				Aliases: []string{"s"},
				Usage:   "Session ID to resume (empty = new session)",
			},
			&cli.BoolFlag{
				Name:    "dangerously-accept-all",
				Aliases: []string{"y"},
				Usage:   "Auto-approve all dangerous tool executions (no confirmation prompts)",
			},
			&cli.IntFlag{
				Name:  "timeout",
				Usage: "Response timeout in seconds",
				Value: 120,
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
	sessionFlag := cmd.String("session")
	acceptAll := cmd.Bool("dangerously-accept-all")

	timeoutSecs := cmd.Int("timeout")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	client, err := wsclient.Dial(ctx, gatewayURL)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}
	defer client.Close()

	// Open or resume session
	cwd, _ := os.Getwd()
	sid, err := client.OpenSession(wsclient.OpenSessionOpts{
		SessionID: sessionFlag,
		RootDir:   cwd,
	})
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	if sessionFlag == "" {
		fmt.Fprintf(os.Stderr, "session: %s\n", sid)
	}

	// Enable accept-all mode if requested
	if acceptAll {
		if err := client.AcceptAllTools(); err != nil {
			return fmt.Errorf("accept all tools: %w", err)
		}
	}

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

		case events.EventPromptRequest:
			var evt events.Event
			if err := json.Unmarshal(frame.Payload, &evt); err != nil {
				continue
			}
			payload, ok := events.GetPromptRequestPayload(evt)
			if !ok {
				continue
			}

			// Ask the user for confirmation on stderr
			fmt.Fprintf(os.Stderr, "\n%s [y/N] ", payload.Label)
			scanner := bufio.NewScanner(os.Stdin)
			cancelled := true
			if scanner.Scan() {
				answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
				cancelled = answer != "y" && answer != "yes"
			}

			if err := client.RespondToPrompt(payload.Token, cancelled); err != nil {
				fmt.Fprintf(os.Stderr, "warning: send prompt response: %v\n", err)
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
