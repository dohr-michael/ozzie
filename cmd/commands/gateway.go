package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/agent"
	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/gateway"
	"github.com/dohr-michael/ozzie/internal/models"
	"github.com/dohr-michael/ozzie/internal/plugins"
	"github.com/dohr-michael/ozzie/internal/sessions"
)

// NewGatewayCommand returns the gateway subcommand.
func NewGatewayCommand() *cli.Command {
	return &cli.Command{
		Name:  "gateway",
		Usage: "Start the Ozzie gateway server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "host",
				Usage: "Host to listen on",
			},
			&cli.IntFlag{
				Name:  "port",
				Usage: "Port to listen on",
			},
		},
		Action: runGateway,
	}
}

func runGateway(_ context.Context, cmd *cli.Command) error {
	// Setup debug logging
	if cmd.Bool("debug") {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	// Load config
	configPath := cmd.String("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Warn("config not found, using defaults", "path", configPath, "error", err)
		cfg = &config.Config{}
		cfg.Gateway.Host = "127.0.0.1"
		cfg.Gateway.Port = 18420
		cfg.Events.BufferSize = 1024
	}

	// CLI flags override config
	if cmd.IsSet("host") {
		cfg.Gateway.Host = cmd.String("host")
	}
	if cmd.IsSet("port") {
		cfg.Gateway.Port = cmd.Int("port")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Event bus
	bus := events.NewBus(cfg.Events.BufferSize)
	defer bus.Close()

	// Model registry
	registry := models.NewRegistry(cfg.Models)

	// Get default model
	chatModel, err := registry.Default(ctx)
	if err != nil {
		return fmt.Errorf("init default model: %w", err)
	}

	// Plugin registry — load WASM plugins + register native tools
	toolRegistry, err := plugins.SetupToolRegistry(ctx, cfg, bus)
	if err != nil {
		return fmt.Errorf("setup tools: %w", err)
	}
	defer toolRegistry.Close(ctx)

	// Wrap dangerous tools with confirmation for interactive gateway
	plugins.WrapRegistryDangerous(toolRegistry, bus)

	allTools := toolRegistry.Tools()
	slog.Info("tools loaded", "count", len(allTools))

	// Agent — empty system_prompt falls back to DefaultSystemPrompt (Ozzie persona)
	runner, err := agent.NewAgent(ctx, chatModel, cfg.Agent.SystemPrompt, allTools)
	if err != nil {
		return fmt.Errorf("init agent: %w", err)
	}

	// Session store
	sessionsDir := filepath.Join(config.OzziePath(), "sessions")
	sessionStore := sessions.NewFileStore(sessionsDir)

	// Event runner
	eventRunner := agent.NewEventRunner(agent.EventRunnerConfig{
		Runner:   runner,
		EventBus: bus,
		Store:    sessionStore,
	})
	defer eventRunner.Close()

	// Gateway server
	server := gateway.NewServer(bus, sessionStore, cfg.Gateway.Host, cfg.Gateway.Port)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	// Wait for signal or error
	select {
	case <-ctx.Done():
		slog.Info("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
