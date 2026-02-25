package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	einoCallbacks "github.com/cloudwego/eino/callbacks"
	"github.com/urfave/cli/v3"

	einoFs "github.com/cloudwego/eino/adk/middlewares/filesystem"
	einoReduction "github.com/cloudwego/eino/adk/middlewares/reduction"

	"github.com/dohr-michael/ozzie/internal/actors"
	"github.com/dohr-michael/ozzie/internal/agent"
	ozzieCallbacks "github.com/dohr-michael/ozzie/internal/callbacks"
	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/gateway"
	"github.com/dohr-michael/ozzie/internal/heartbeat"
	"github.com/dohr-michael/ozzie/internal/memory"
	"github.com/dohr-michael/ozzie/internal/models"
	"github.com/dohr-michael/ozzie/internal/plugins"
	"github.com/dohr-michael/ozzie/internal/scheduler"
	"github.com/dohr-michael/ozzie/internal/sessions"
	"github.com/dohr-michael/ozzie/internal/skills"
	"github.com/dohr-michael/ozzie/internal/storage"
	"github.com/dohr-michael/ozzie/internal/tasks"
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
	// Load config
	configPath := cmd.String("config")
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Warn("config not found, using defaults", "path", configPath, "error", err)
		cfg = &config.Config{}
		cfg.Gateway.Host = "127.0.0.1"
		cfg.Gateway.Port = 18420
		cfg.Events.BufferSize = 1024
		cfg.Events.LogLevel = "info"
	}

	// Setup log level: config value, with --debug CLI override
	logLevel := resolveLogLevel(cfg.Events.LogLevel)
	if cmd.Bool("debug") {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

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

	// Register Eino callbacks → event bus bridge
	cbHandler := ozzieCallbacks.NewEventBusHandler(bus, events.SourceAgent)
	einoCallbacks.AppendGlobalHandlers(cbHandler)

	// Event persistence
	logsDir := filepath.Join(config.OzziePath(), "logs")
	eventLogger := storage.NewEventLogger(logsDir, bus)
	defer eventLogger.Close()

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

	// Tool permissions — global auto-approved tools from config
	toolPerms := plugins.NewToolPermissions(cfg.Tools.AllowedDangerous)

	// Sandbox guard — validates command content in autonomous mode (before dangerous wrapper)
	if cfg.Sandbox.IsSandboxEnabled() {
		plugins.WrapRegistrySandbox(toolRegistry, cfg.Sandbox.AllowedPaths)
	}

	// Wrap dangerous tools with confirmation for interactive gateway
	plugins.WrapRegistryDangerous(toolRegistry, bus, toolPerms)

	// Skill registry — load declarative skills and register as tools
	skillRegistry := skills.NewRegistry()
	for _, dir := range cfg.Skills.Dirs {
		if err := skillRegistry.LoadDir(dir); err != nil {
			slog.Warn("failed to load skills", "dir", dir, "error", err)
		}
	}

	// Verifier for acceptance criteria
	verifier := skills.NewVerifier(registry)

	skillRunCfg := skills.RunnerConfig{
		ModelRegistry: registry,
		ToolRegistry:  toolRegistry,
		EventBus:      bus,
		Verifier:      verifier,
	}
	skillDescs := make(map[string]string)
	for _, sk := range skillRegistry.All() {
		skillTool := skills.NewSkillTool(sk, skillRunCfg)
		manifest := skills.SkillToManifest(sk)
		if err := toolRegistry.RegisterNative(sk.Name, skillTool, manifest); err != nil {
			slog.Warn("register skill", "name", sk.Name, "error", err)
		}
		skillDescs[sk.Name] = sk.Description
	}
	slog.Info("skills loaded", "count", len(skillRegistry.All()))

	// Session store
	sessionsDir := filepath.Join(config.OzziePath(), "sessions")
	sessionStore := sessions.NewFileStore(sessionsDir)

	// Task store
	tasksDir := filepath.Join(config.OzziePath(), "tasks")
	taskStore := tasks.NewFileStore(tasksDir)

	// Crash recovery — re-queue interrupted tasks
	recovered, recoverErr := tasks.RecoverTasks(taskStore)
	if recoverErr != nil {
		slog.Warn("task recovery", "error", recoverErr)
	}
	if recovered > 0 {
		slog.Info("recovered interrupted tasks", "count", recovered)
	}

	// Heartbeat writer
	hbWriter := heartbeat.NewWriter(filepath.Join(config.OzziePath(), "heartbeat.json"))
	hbWriter.Start()
	defer hbWriter.Stop()

	// Register update_session tool (needs session store, so registered here)
	updateSessionTool := plugins.NewUpdateSessionTool(sessionStore)
	if err := toolRegistry.RegisterNative("update_session", updateSessionTool, plugins.UpdateSessionManifest()); err != nil {
		slog.Warn("failed to register update_session tool", "error", err)
	}

	// Agent — persona from SOUL.md or DefaultPersona fallback (layer 1)
	persona := agent.LoadPersona()
	slog.Debug("loaded persona", "length", len(persona), "persona", persona)

	// Filesystem middleware — provides ls, read_file, write_file, edit_file, glob, grep via Eino ADK
	fsBackend := plugins.NewOzzieBackend()
	fsMw, err := einoFs.NewMiddleware(ctx, &einoFs.Config{
		Backend:                          fsBackend,
		WithoutLargeToolResultOffloading: true, // offloading handled by reduction middleware below
	})
	if err != nil {
		return fmt.Errorf("init filesystem middleware: %w", err)
	}

	// Reduction middleware — clears old tool results and offloads large ones to filesystem
	reductionMw, err := einoReduction.NewToolResultMiddleware(ctx, &einoReduction.ToolResultConfig{
		Backend:                fsBackend,
		ClearingTokenThreshold: 20000,
		KeepRecentTokens:       40000,
		OffloadingTokenLimit:   20000,
	})
	if err != nil {
		return fmt.Errorf("init reduction middleware: %w", err)
	}

	// SubAgent middleware — injects SubAgentInstructions (tool reference + workflow)
	subAgentMw := agent.NewSubAgentMiddleware()

	// Task middlewares — subagent instructions + filesystem + reduction for sub-agents (no context middleware)
	taskMiddlewares := []adk.AgentMiddleware{subAgentMw, fsMw, reductionMw}

	// Skill executor for direct skill tasks
	skillExecutor := skills.NewPoolSkillExecutor(skillRegistry, skillRunCfg)

	// Actor pool — capacity-aware LLM orchestration (replaces WorkerPool)
	pool := actors.NewActorPool(actors.ActorPoolConfig{
		Providers:           cfg.Models.Providers,
		Store:               taskStore,
		Bus:                 bus,
		Models:              registry,
		ToolRegistry:        toolRegistry,
		AutonomyDefault:     cfg.Agent.Coordinator.DefaultLevel,
		MaxValidationRounds: cfg.Agent.Coordinator.MaxValidationRounds,
		SkillRunner:         skillExecutor,
		TaskMiddlewares:     taskMiddlewares,
	})
	pool.Start()
	defer pool.Stop()

	// Schedule store — persistent dynamic schedule entries
	schedulesDir := filepath.Join(config.OzziePath(), "schedules")
	scheduleStore := scheduler.NewScheduleStore(schedulesDir)

	// Extract skill schedule info for the scheduler (avoids import cycle)
	var schedSkills []scheduler.SkillScheduleInfo
	for _, sk := range skillRegistry.All() {
		if !sk.Triggers.HasScheduleTrigger() {
			continue
		}
		info := scheduler.SkillScheduleInfo{
			Name: sk.Name,
			Cron: sk.Triggers.Cron,
		}
		if sk.Triggers.OnEvent != nil {
			info.OnEvent = &scheduler.EventTrigger{
				Event:  sk.Triggers.OnEvent.Event,
				Filter: sk.Triggers.OnEvent.Filter,
			}
		}
		schedSkills = append(schedSkills, info)
	}

	// Scheduler — cron + event-triggered + dynamic schedule execution
	sched := scheduler.New(scheduler.Config{
		Pool:   pool,
		Bus:    bus,
		Skills: schedSkills,
		Store:  scheduleStore,
	})
	sched.Start()
	defer sched.Stop()

	// Memory store + optional vector embedding
	memoryDir := filepath.Join(config.OzziePath(), "memory")
	memoryStore := memory.NewFileStore(memoryDir)

	var vectorStore *memory.VectorStore
	var pipeline *memory.Pipeline
	if cfg.Embedding.IsEnabled() {
		embedder, embedErr := memory.NewEmbedder(ctx, cfg.Embedding)
		if embedErr != nil {
			slog.Warn("embedding disabled: failed to create embedder", "error", embedErr)
		} else {
			vs, vsErr := memory.NewVectorStore(ctx, memoryDir, embedder)
			if vsErr != nil {
				slog.Warn("embedding disabled: failed to create vector store", "error", vsErr)
			} else {
				vectorStore = vs
				queueSize := cfg.Embedding.QueueSize
				if queueSize <= 0 {
					queueSize = 100
				}
				pipeline = memory.NewPipeline(vectorStore, queueSize)
				pipeline.Start(ctx)
				defer pipeline.Stop()

				// Async startup reindex
				go func() {
					if _, err := memory.Reindex(ctx, memoryStore, vectorStore); err != nil {
						slog.Warn("startup reindex failed", "error", err)
					}
				}()
				slog.Info("semantic memory enabled", "driver", cfg.Embedding.Driver, "model", cfg.Embedding.Model)
			}
		}
	}

	memoryRetriever := memory.NewHybridRetriever(memoryStore, vectorStore)

	// Register memory tools
	storeMemTool := plugins.NewStoreMemoryTool(memoryStore, pipeline)
	if err := toolRegistry.RegisterNative("store_memory", storeMemTool, plugins.StoreMemoryManifest()); err != nil {
		slog.Warn("failed to register store_memory tool", "error", err)
	}

	queryMemTool := plugins.NewQueryMemoriesTool(memoryRetriever)
	if err := toolRegistry.RegisterNative("query_memories", queryMemTool, plugins.QueryMemoriesManifest()); err != nil {
		slog.Warn("failed to register query_memories tool", "error", err)
	}

	forgetMemTool := plugins.NewForgetMemoryTool(memoryStore, pipeline)
	if err := toolRegistry.RegisterNative("forget_memory", forgetMemTool, plugins.ForgetMemoryManifest()); err != nil {
		slog.Warn("failed to register forget_memory tool", "error", err)
	}

	// Register task tools
	submitTool := plugins.NewSubmitTaskTool(pool, cfg.Agent.Coordinator.DefaultLevel)
	if err := toolRegistry.RegisterNative("submit_task", submitTool, plugins.SubmitTaskManifest()); err != nil {
		slog.Warn("failed to register submit_task tool", "error", err)
	}

	checkTool := plugins.NewCheckTaskTool(taskStore)
	if err := toolRegistry.RegisterNative("check_task", checkTool, plugins.CheckTaskManifest()); err != nil {
		slog.Warn("failed to register check_task tool", "error", err)
	}

	cancelTool := plugins.NewCancelTaskTool(pool)
	if err := toolRegistry.RegisterNative("cancel_task", cancelTool, plugins.CancelTaskManifest()); err != nil {
		slog.Warn("failed to register cancel_task tool", "error", err)
	}

	planTool := plugins.NewPlanTaskTool(pool)
	if err := toolRegistry.RegisterNative("plan_task", planTool, plugins.PlanTaskManifest()); err != nil {
		slog.Warn("failed to register plan_task tool", "error", err)
	}

	listTasksTool := plugins.NewListTasksTool(taskStore)
	if err := toolRegistry.RegisterNative("list_tasks", listTasksTool, plugins.ListTasksManifest()); err != nil {
		slog.Warn("failed to register list_tasks tool", "error", err)
	}

	// Register schedule tools
	scheduleTaskTool := plugins.NewScheduleTaskTool(sched, bus)
	if err := toolRegistry.RegisterNative("schedule_task", scheduleTaskTool, plugins.ScheduleTaskManifest()); err != nil {
		slog.Warn("failed to register schedule_task tool", "error", err)
	}

	unscheduleTaskTool := plugins.NewUnscheduleTaskTool(sched, bus)
	if err := toolRegistry.RegisterNative("unschedule_task", unscheduleTaskTool, plugins.UnscheduleTaskManifest()); err != nil {
		slog.Warn("failed to register unschedule_task tool", "error", err)
	}

	listSchedulesTool := plugins.NewListSchedulesTool(sched)
	if err := toolRegistry.RegisterNative("list_schedules", listSchedulesTool, plugins.ListSchedulesManifest()); err != nil {
		slog.Warn("failed to register list_schedules tool", "error", err)
	}

	// Register validation tools (coordinator pattern)
	requestValidationTool := plugins.NewRequestValidationTool()
	if err := toolRegistry.RegisterNative("request_validation", requestValidationTool, plugins.RequestValidationManifest()); err != nil {
		slog.Warn("failed to register request_validation tool", "error", err)
	}

	replyTaskTool := plugins.NewReplyTaskTool(pool)
	if err := toolRegistry.RegisterNative("reply_task", replyTaskTool, plugins.ReplyTaskManifest()); err != nil {
		slog.Warn("failed to register reply_task tool", "error", err)
	}

	// ToolSet: all native/built-in tools are always active,
	// only WASM plugin tools require dynamic activation.
	// Must be computed AFTER all native tools are registered.
	coreTools := toolRegistry.NativeToolNames()
	toolSet := agent.NewToolSet(coreTools, toolRegistry.ToolNames())

	// Register activate_tools meta-tool (needs toolSet + toolRegistry)
	activateTool := plugins.NewActivateToolsTool(toolSet, toolRegistry)
	if err := toolRegistry.RegisterNative("activate_tools", activateTool, plugins.ActivateToolsManifest()); err != nil {
		slog.Warn("failed to register activate_tools tool", "error", err)
	}

	slog.Info("tools loaded", "count", len(toolRegistry.ToolNames()))

	// Build full tool descriptions for prompt composer
	allToolDescs := toolRegistry.AllToolDescriptions()

	// Context middleware — injects dynamic context (instructions, tools, session, memories)
	contextMw := agent.NewContextMiddleware(agent.ContextMiddlewareConfig{
		CustomInstructions:  cfg.Agent.SystemPrompt,
		AllToolDescriptions: allToolDescs,
		SkillDescriptions:   skillDescs,
		Store:               sessionStore,
		ToolSet:             toolSet,
		Retriever:           memoryRetriever,
	})

	var middlewares []adk.AgentMiddleware
	middlewares = append(middlewares, fsMw, reductionMw, contextMw)

	// AgentFactory (replaces single runner — creates fresh runner per turn)
	factory := agent.NewAgentFactory(chatModel, persona, middlewares)

	// Cost tracker — accumulates token usage per session
	costTracker := storage.NewCostTracker(bus, sessionStore)
	defer costTracker.Close()

	// Event runner with dynamic tool selection and actor pool integration
	eventRunner := agent.NewEventRunner(agent.EventRunnerConfig{
		Factory:         factory,
		ToolSet:         toolSet,
		Registry:        toolRegistry,
		EventBus:        bus,
		Store:           sessionStore,
		TaskStore:       newTaskStoreAdapter(taskStore),
		Pool:            actors.NewPoolAdapter(pool),
		DefaultProvider: registry.DefaultName(),
		ContextWindow:   registry.DefaultContextWindow(),
	})
	defer eventRunner.Close()

	// Gateway server
	server := gateway.NewServer(bus, sessionStore, cfg.Gateway.Host, cfg.Gateway.Port, toolPerms)

	// Connect task handler to gateway (WS + HTTP task operations)
	taskHandler := gateway.NewWSTaskHandler(pool)
	server.SetTaskHandler(taskHandler)

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

// taskStoreAdapter adapts tasks.Store to agent.TaskStore (avoids import cycle).
type taskStoreAdapter struct {
	store tasks.Store
}

func newTaskStoreAdapter(s tasks.Store) *taskStoreAdapter {
	return &taskStoreAdapter{store: s}
}

func resolveLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (a *taskStoreAdapter) LoadMailbox(taskID string) ([]agent.TaskMailboxMessage, error) {
	msgs, err := a.store.LoadMailbox(taskID)
	if err != nil {
		return nil, err
	}
	result := make([]agent.TaskMailboxMessage, len(msgs))
	for i, m := range msgs {
		result[i] = agent.TaskMailboxMessage{
			Type:    m.Type,
			Content: m.Content,
		}
	}
	return result, nil
}
