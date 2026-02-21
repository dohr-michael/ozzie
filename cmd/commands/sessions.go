package commands

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

// NewSessionsCommand returns the sessions subcommand (stub).
func NewSessionsCommand() *cli.Command {
	return &cli.Command{
		Name:  "sessions",
		Usage: "Manage agent sessions",
		Action: func(_ context.Context, _ *cli.Command) error {
			fmt.Println("Sessions not yet implemented")
			return nil
		},
	}
}
