package commands

import (
	"context"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/events"
	ozziemcp "github.com/dohr-michael/ozzie/internal/mcp"
	"github.com/dohr-michael/ozzie/internal/plugins"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewMCPServeCommand returns the mcp-serve subcommand.
func NewMCPServeCommand() *cli.Command {
	return &cli.Command{
		Name:  "mcp-serve",
		Usage: "Expose Ozzie tools as an MCP server (stdio)",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:      "filter",
				UsageText: "Plugin or tool name to expose (empty = all)",
			},
		},
		Action: runMCPServe,
	}
}

func runMCPServe(_ context.Context, cmd *cli.Command) error {
	// Setup logging to stderr (stdout is used for MCP stdio transport)
	if cmd.Bool("debug") {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	}

	// Load config
	configPath := cmd.String("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Debug("config not found, using defaults", "path", configPath, "error", err)
		cfg = &config.Config{}
	}

	ctx := context.Background()

	// Minimal event bus (needed for plugin host functions)
	bus := events.NewBus(64)
	defer bus.Close()

	// Load tools without dangerous wrappers — MCP clients handle their own confirmations
	toolRegistry, err := plugins.SetupToolRegistry(ctx, cfg, bus)
	if err != nil {
		return err
	}
	defer toolRegistry.Close(ctx)

	// Optional filter — use StringArg for urfave/cli v3 Arguments
	filter := cmd.StringArg("filter")

	slog.Debug("starting MCP server", "filter", filter, "tools", len(toolRegistry.ToolNames()))

	// Create and run MCP server
	server := ozziemcp.NewMCPServer(toolRegistry, filter)
	return server.Run(ctx, &mcpsdk.StdioTransport{})
}
