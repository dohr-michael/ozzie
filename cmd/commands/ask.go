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
			&cli.StringFlag{
				Name:    "working-dir",
				Aliases: []string{"w"},
				Usage:   "Working directory for the session (default: current directory)",
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

	workDir := cmd.String("working-dir")
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	timeoutSecs := cmd.Int("timeout")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	client, err := wsclient.Dial(ctx, gatewayURL)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}
	defer client.Close()

	sid, err := client.OpenSession(wsclient.OpenSessionOpts{
		SessionID: sessionFlag,
		RootDir:   workDir,
	})
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	if sessionFlag == "" {
		fmt.Fprintf(os.Stderr, "session: %s\n", sid)
	}

	opts := []tui.AppOption{
		tui.WithInitialMessage(message),
		tui.WithAutoQuit(),
	}
	if acceptAll {
		opts = append(opts, tui.WithAcceptAll())
	}

	model := tui.NewApp(client, sid, opts...)
	p := tea.NewProgram(model)

	// Simple read loop (no reconnection for single-shot).
	go func() {
		for {
			frame, err := client.ReadFrame()
			if err != nil {
				return
			}
			if msg := tui.Project(frame); msg != nil {
				p.Send(msg)
			}
		}
	}()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
