package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/sessions"
)

// NewSessionsCommand returns the sessions subcommand.
func NewSessionsCommand() *cli.Command {
	return &cli.Command{
		Name:  "sessions",
		Usage: "Manage agent sessions",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all sessions",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "json", Usage: "Output raw JSON"},
				},
				Action: runSessionsList,
			},
			{
				Name:      "show",
				Usage:     "Show messages in a session",
				ArgsUsage: "<name|id>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "json", Usage: "Output raw JSON"},
				},
				Action: runSessionsShow,
			},
		},
		DefaultCommand: "list",
	}
}

func newStore() *sessions.FileStore {
	return sessions.NewFileStore(filepath.Join(config.OzziePath(), "sessions"))
}

func runSessionsList(_ context.Context, cmd *cli.Command) error {
	store := newStore()

	list, err := store.List()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if cmd.Bool("json") {
		return json.NewEncoder(os.Stdout).Encode(list)
	}

	if len(list) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tMESSAGES\tUPDATED\tTITLE")
	for _, s := range list {
		title := s.Title
		if title == "" {
			title = "-"
		}
		displayName := s.Name
		if displayName == "" {
			displayName = s.ID
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
			displayName,
			s.Status,
			s.MessageCount,
			s.UpdatedAt.Format("2006-01-02 15:04"),
			title,
		)
	}
	return w.Flush()
}

func runSessionsShow(_ context.Context, cmd *cli.Command) error {
	ref := cmd.Args().First()
	if ref == "" {
		return fmt.Errorf("usage: ozzie sessions show <name|id>")
	}

	store := newStore()

	msgs, err := store.LoadMessages(ref)
	if err != nil {
		return fmt.Errorf("load messages: %w", err)
	}

	if cmd.Bool("json") {
		return json.NewEncoder(os.Stdout).Encode(msgs)
	}

	if len(msgs) == 0 {
		fmt.Println("No messages in this session.")
		return nil
	}

	for _, m := range msgs {
		fmt.Printf("[%s] %s: %s\n", m.Ts.Format("15:04:05"), m.Role, m.Content)
	}
	return nil
}
