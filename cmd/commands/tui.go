package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/clients/tui"
	wsclient "github.com/dohr-michael/ozzie/clients/ws"
)

// NewTUICommand returns the tui subcommand.
func NewTUICommand() *cli.Command {
	return &cli.Command{
		Name:  "tui",
		Usage: "Launch the interactive terminal UI",
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
		},
		Action: runTUI,
	}
}

func runTUI(_ context.Context, cmd *cli.Command) error {
	gatewayURL := cmd.String("gateway")
	sessionFlag := cmd.String("session")

	ctx := context.Background()

	client, err := wsclient.Dial(ctx, gatewayURL)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}

	cwd, _ := os.Getwd()
	sid, err := client.OpenSession(wsclient.OpenSessionOpts{
		SessionID: sessionFlag,
		RootDir:   cwd,
	})
	if err != nil {
		client.Close()
		return fmt.Errorf("open session: %w", err)
	}

	model := tui.NewApp(client, sid)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Goroutine: read WS frames with reconnection.
	go readLoop(ctx, p, client, gatewayURL, sid, cwd)

	if _, err := p.Run(); err != nil {
		client.Close()
		return fmt.Errorf("tui: %w", err)
	}

	client.Close()
	return nil
}

// readLoop reads WS frames and reconnects on failure with exponential backoff.
func readLoop(ctx context.Context, p *tea.Program, client *wsclient.Client, gatewayURL, sessionID, cwd string) {
	for {
		frame, err := client.ReadFrame()
		if err != nil {
			p.Send(tui.DisconnectedMsg{Err: err})

			// Attempt reconnection with backoff.
			newClient := reconnect(ctx, p, gatewayURL, sessionID, cwd)
			if newClient == nil {
				return // context cancelled, give up
			}
			client = newClient
			continue
		}
		if msg := tui.Project(frame); msg != nil {
			p.Send(msg)
		}
	}
}

func reconnect(ctx context.Context, p *tea.Program, gatewayURL, sessionID, cwd string) *wsclient.Client {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}

		client, err := wsclient.Dial(ctx, gatewayURL)
		if err != nil {
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		_, err = client.OpenSession(wsclient.OpenSessionOpts{
			SessionID: sessionID,
			RootDir:   cwd,
		})
		if err != nil {
			client.Close()
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		p.Send(tui.ConnectedMsg{SessionID: sessionID, Client: client})
		return client
	}
}
