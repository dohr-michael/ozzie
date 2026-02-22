package commands

import (
	"context"
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
				Name:   "list",
				Usage:  "List all sessions",
				Action: runSessionsList,
			},
			{
				Name:      "show",
				Usage:     "Show messages in a session",
				ArgsUsage: "<session_id>",
				Action:    runSessionsShow,
			},
		},
		DefaultCommand: "list",
	}
}

func newStore() *sessions.FileStore {
	return sessions.NewFileStore(filepath.Join(config.OzziePath(), "sessions"))
}

func runSessionsList(_ context.Context, _ *cli.Command) error {
	store := newStore()

	list, err := store.List()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(list) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tMESSAGES\tUPDATED\tTITLE")
	for _, s := range list {
		title := s.Title
		if title == "" {
			title = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
			s.ID,
			s.Status,
			s.MessageCount,
			s.UpdatedAt.Format("2006-01-02 15:04"),
			title,
		)
	}
	return w.Flush()
}

func runSessionsShow(_ context.Context, cmd *cli.Command) error {
	sessionID := cmd.Args().First()
	if sessionID == "" {
		return fmt.Errorf("usage: ozzie sessions show <session_id>")
	}

	store := newStore()

	msgs, err := store.LoadMessages(sessionID)
	if err != nil {
		return fmt.Errorf("load messages: %w", err)
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
