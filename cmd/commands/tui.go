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
			&cli.StringFlag{
				Name:    "working-dir",
				Aliases: []string{"w"},
				Usage:   "Working directory for the session (default: current directory)",
			},
			&cli.BoolFlag{
				Name:  "insecure",
				Usage: "Skip authentication (for dev mode)",
			},
		},
		Action: runTUI,
	}
}

func runTUI(_ context.Context, cmd *cli.Command) error {
	gatewayURL := cmd.String("gateway")
	sessionFlag := cmd.String("session")

	workDir := cmd.String("working-dir")
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Auto-discover local token for authentication
	var dialOpts []wsclient.DialOption
	if !cmd.Bool("insecure") {
		if token := wsclient.DiscoverLocalToken(); token != "" {
			dialOpts = append(dialOpts, wsclient.WithToken(token))
		}
	}

	ctx := context.Background()

	client, err := wsclient.Dial(ctx, gatewayURL, dialOpts...)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}

	sid, err := client.OpenSession(wsclient.OpenSessionOpts{
		SessionID: sessionFlag,
		RootDir:   workDir,
	})
	if err != nil {
		client.Close()
		return fmt.Errorf("open session: %w", err)
	}

	var opts []tui.AppOption
	if sessionFlag != "" {
		msgs, err := client.LoadMessages(10)
		if err == nil && len(msgs) > 0 {
			history := make([]tui.HistoryMessage, len(msgs))
			for i, m := range msgs {
				history[i] = tui.HistoryMessage{Role: m.Role, Content: m.Content}
			}
			opts = append(opts, tui.WithHistory(history))
		}
	}

	model := tui.NewApp(client, sid, opts...)
	p := tea.NewProgram(model)

	// Goroutine: read WS frames with reconnection.
	go readLoop(ctx, p, client, gatewayURL, sid, workDir, dialOpts)

	if _, err := p.Run(); err != nil {
		client.Close()
		return fmt.Errorf("tui: %w", err)
	}

	client.Close()
	return nil
}

// readLoop reads WS frames and reconnects on failure with exponential backoff.
func readLoop(ctx context.Context, p *tea.Program, client *wsclient.Client, gatewayURL, sessionID, cwd string, dialOpts []wsclient.DialOption) {
	for {
		frame, err := client.ReadFrame()
		if err != nil {
			p.Send(tui.DisconnectedMsg{Err: err})

			// Attempt reconnection with backoff.
			newClient := reconnect(ctx, p, gatewayURL, sessionID, cwd, dialOpts)
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

func reconnect(ctx context.Context, p *tea.Program, gatewayURL, sessionID, cwd string, dialOpts []wsclient.DialOption) *wsclient.Client {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}

		client, err := wsclient.Dial(ctx, gatewayURL, dialOpts...)
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
