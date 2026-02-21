package commands

import (
	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/config"
)

// NewRootCommand returns the top-level CLI command.
func NewRootCommand() *cli.Command {
	return &cli.Command{
		Name:  "ozzie",
		Usage: "Your personal AI agent operating system",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to config file",
				Value:   config.ConfigPath(),
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug logging",
			},
		},
		Commands: []*cli.Command{
			NewWakeCommand(),
			NewGatewayCommand(),
			NewAskCommand(),
			NewTUICommand(),
			NewStatusCommand(),
			NewSessionsCommand(),
			NewMCPServeCommand(),
		},
	}
}
