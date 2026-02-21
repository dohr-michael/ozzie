package commands

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

// NewStatusCommand returns the status subcommand (stub).
func NewStatusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show Ozzie gateway status",
		Action: func(_ context.Context, _ *cli.Command) error {
			fmt.Println("Status not yet implemented")
			return nil
		},
	}
}
