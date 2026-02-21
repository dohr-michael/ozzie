package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/dohr-michael/ozzie/cmd/commands"
	"github.com/dohr-michael/ozzie/internal/config"
)

func main() {
	if err := config.LoadDotenv(config.DotenvPath()); err != nil {
		slog.Warn("failed to load .env", "error", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cmd := commands.NewRootCommand()
	if err := cmd.Run(ctx, os.Args); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}
