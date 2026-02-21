package commands

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

// NewTUICommand returns the tui subcommand (stub).
func NewTUICommand() *cli.Command {
	return &cli.Command{
		Name:  "tui",
		Usage: "Launch the interactive TUI",
		Action: func(_ context.Context, _ *cli.Command) error {
			fmt.Println("TUI not yet implemented")
			return nil
		},
	}
}
