package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/core/brain"
	"github.com/dohr-michael/ozzie/internal/brain/actors"
	"github.com/dohr-michael/ozzie/internal/agent"
	"github.com/dohr-michael/ozzie/internal/auth"
	"github.com/dohr-michael/ozzie/internal/brain/conscience"
	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/eyes"
	"github.com/dohr-michael/ozzie/internal/core/events"
	ozzieGateway "github.com/dohr-michael/ozzie/internal/gateway"
	layeredctx "github.com/dohr-michael/ozzie/internal/brain/memory/layered"
	"github.com/dohr-michael/ozzie/internal/models"
	"github.com/dohr-michael/ozzie/internal/hands"
	"github.com/dohr-michael/ozzie/internal/policy"
	"github.com/dohr-michael/ozzie/internal/scheduler"
	"github.com/dohr-michael/ozzie/internal/secrets"
	"github.com/dohr-michael/ozzie/internal/sessions"
	"github.com/dohr-michael/ozzie/internal/brain/skills"
	"github.com/dohr-michael/ozzie/internal/tasks"
	"github.com/dohr-michael/ozzie/pkg/memory"
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
			&cli.BoolFlag{
				Name:  "insecure",
				Usage: "Disable authentication (dev mode only)",
			},
		},
		Action: runGateway,
	}
}

// gateway holds all components wired during startup.
type gateway struct {
	cmd *cli.Command
	ctx context.Context

	// Config
	cfg      *config.Config
	kr       *secrets.KeyRing
	reloader *config.Reloader

	// Infra
	bus events.EventBus

	// Models
	registry    *models.Registry
	chatModel   model.ToolCallingChatModel
	defaultTier brain.ModelTier

	// Tools
	toolRegistry *hands.ToolRegistry
	toolPerms    *conscience.ToolPermissions
	toolSet      *brain.ToolSet
	tmpDir       string

	// Skills
	skillRegistry *skills.Registry
	skillRunCfg   skills.RunnerConfig
	skillExecutor *skills.PoolSkillExecutor
	skillDescs    map[string]string

	// Stores
	sessionStore sessions.Store
	taskStore    tasks.Store

	// Memory
	memoryStore     *memory.SQLiteStore
	memoryRetriever *memory.HybridRetriever
	pipeline        *memory.Pipeline
	embFingerprint  string

	// Policy
	policyResolver *policy.PolicyResolver
	pairingStore   *policy.PairingStore

	// Runtime
	pool  *actors.ActorPool
	sched *scheduler.Scheduler

	// Agent
	persona     string
	factory     *agent.AgentFactory
	eventRunner *agent.EventRunner
	taskMws     []any
	layered     *layeredctx.Manager

	// Connectors
	connectorManager *eyes.Manager

	closers []func()
}

func runGateway(parentCtx context.Context, cmd *cli.Command) error {
	g := &gateway{cmd: cmd}
	if err := g.loadConfig(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(parentCtx, os.Interrupt)
	defer stop()
	g.ctx = ctx
	g.startSIGHUP()

	if err := g.initInfra(); err != nil {
		return err
	}
	defer g.close()
	if err := g.initModels(); err != nil {
		return err
	}
	if err := g.initToolPipeline(); err != nil {
		return err
	}
	if err := g.initSkills(); err != nil {
		return err
	}
	if err := g.initStores(); err != nil {
		return err
	}
	g.initPolicy()
	if err := g.initMemory(); err != nil {
		return err
	}
	if err := g.initRuntime(); err != nil {
		return err
	}
	g.registerTools()
	if err := g.initAgent(); err != nil {
		return err
	}
	g.initConnectors()
	return g.serve()
}

// serve starts the HTTP/WS gateway and blocks until shutdown.
func (g *gateway) serve() error {
	// Auth — local token (requires keyring for encryption)
	var authenticator auth.Authenticator
	if !g.cmd.Bool("insecure") && g.kr != nil {
		tokenPath := filepath.Join(config.OzziePath(), ".local_token")
		localAuth, err := auth.NewLocalAuth(tokenPath, g.kr.CurrentRecipient())
		if err != nil {
			return fmt.Errorf("init auth: %w", err)
		}
		authenticator = localAuth
		g.closers = append(g.closers, func() { os.Remove(tokenPath) })
		slog.Info("auth enabled", "mode", "local_token")
	} else if !g.cmd.Bool("insecure") && g.kr == nil {
		slog.Warn("auth disabled: no keyring available (run ozzie wake to create one)")
	} else {
		slog.Warn("auth disabled (--insecure mode)")
	}

	// Gateway server
	server := ozzieGateway.NewServer(g.bus, g.sessionStore, g.cfg.Gateway.Host, g.cfg.Gateway.Port, g.toolPerms, authenticator)

	// Enable secret encryption on the WS hub
	if g.kr != nil {
		server.SetSecretEncryptor(g.kr.CurrentRecipient())
	}

	// Connect task handler to gateway (WS + HTTP task operations)
	taskHandler := ozzieGateway.NewWSTaskHandler(g.pool)
	server.SetTaskHandler(taskHandler)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	// Wait for signal or error
	select {
	case <-g.ctx.Done():
		slog.Info("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// close runs all registered closers in reverse order.
func (g *gateway) close() {
	for i := len(g.closers) - 1; i >= 0; i-- {
		g.closers[i]()
	}
}

// extractorLLMAdapter adapts a ToolCallingChatModel to memory.LLMSummarizer.
type extractorLLMAdapter struct {
	chatModel model.ToolCallingChatModel
}

func (a *extractorLLMAdapter) Summarize(ctx context.Context, prompt string) (string, error) {
	resp, err := a.chatModel.Generate(ctx, []*schema.Message{{Role: schema.User, Content: prompt}})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

