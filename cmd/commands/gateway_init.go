package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cloudwego/eino/adk"
	einoCallbacks "github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"

	einoFs "github.com/cloudwego/eino/adk/middlewares/filesystem"
	einoReduction "github.com/cloudwego/eino/adk/middlewares/reduction"

	"github.com/dohr-michael/ozzie/internal/brain"
	"github.com/dohr-michael/ozzie/internal/brain/actors"
	"github.com/dohr-michael/ozzie/internal/agent"
	"github.com/dohr-michael/ozzie/internal/brain/conscience"
	"github.com/dohr-michael/ozzie/internal/brain/introspection"
	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/eyes"
	"github.com/dohr-michael/ozzie/internal/eyes/discord"
	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/dohr-michael/ozzie/internal/heartbeat"
	"github.com/dohr-michael/ozzie/internal/membridge"
	layeredctx "github.com/dohr-michael/ozzie/internal/brain/memory/layered"
	"github.com/dohr-michael/ozzie/internal/models"
	"github.com/dohr-michael/ozzie/internal/hands"
	"github.com/dohr-michael/ozzie/internal/policy"
	"github.com/dohr-michael/ozzie/internal/core/prompt"
	"github.com/dohr-michael/ozzie/internal/scheduler"
	"github.com/dohr-michael/ozzie/internal/secrets"
	"github.com/dohr-michael/ozzie/internal/sessions"
	"github.com/dohr-michael/ozzie/internal/brain/skills"
	"github.com/dohr-michael/ozzie/internal/tasks"
	"github.com/dohr-michael/ozzie/pkg/connector"
	"github.com/dohr-michael/ozzie/pkg/memory"
	memtools "github.com/dohr-michael/ozzie/pkg/memory/tools"
)

// loadConfig initializes the keyring, loads JSONC config, applies CLI
// overrides, sets up the log level, and creates a config reloader.
func (g *gateway) loadConfig() error {
	// KeyRing — loaded before config (does not depend on config)
	kr, krErr := secrets.NewKeyRing()
	if krErr != nil {
		slog.Warn("keyring not loaded, secret decryption disabled", "error", krErr)
	}
	if kr == nil {
		slog.Info("no .age/ keyring found, secrets will not be decrypted")
	}
	g.kr = kr

	// Decrypt function for config loader
	var decryptFn config.DecryptFunc
	if kr != nil {
		decryptFn = kr.DecryptValue
	}

	// Load config with decryption
	configPath := g.cmd.String("config")
	var loadOpts []config.LoadOption
	if decryptFn != nil {
		loadOpts = append(loadOpts, config.WithDecrypt(decryptFn))
	}
	cfg, err := config.Load(configPath, loadOpts...)
	if err != nil {
		slog.Warn("config not found, using defaults", "path", configPath, "error", err)
		cfg = &config.Config{}
		cfg.Gateway.Host = "127.0.0.1"
		cfg.Gateway.Port = 18420
		cfg.Events.BufferSize = 1024
		cfg.Events.LogLevel = "info"
	}
	g.cfg = cfg

	// Setup log level: config value, with --debug CLI override
	logLevel := introspection.ResolveLogLevel(cfg.Events.LogLevel)
	if g.cmd.Bool("debug") {
		logLevel = slog.LevelDebug
	}
	introspection.SetupLogger(logLevel)

	// CLI flags override config
	if g.cmd.IsSet("host") {
		cfg.Gateway.Host = g.cmd.String("host")
	}
	if g.cmd.IsSet("port") {
		cfg.Gateway.Port = g.cmd.Int("port")
	}

	// Config reloader with decrypt
	g.reloader = config.NewReloader(configPath, config.DotenvPath(), cfg, decryptFn)
	return nil
}

// startSIGHUP launches a goroutine that triggers config hot-reload on SIGHUP.
func (g *gateway) startSIGHUP() {
	sighupCh := make(chan os.Signal, 1)
	signal.Notify(sighupCh, syscall.SIGHUP)
	go func() {
		for range sighupCh {
			if err := g.reloader.Reload(); err != nil {
				slog.Error("config reload failed", "error", err)
			}
		}
	}()
}

// initInfra creates the event bus, wires Eino callbacks, starts the event
// logger, and validates provider capabilities.
func (g *gateway) initInfra() error {
	// Event bus
	g.bus = events.NewBus(g.cfg.Events.BufferSize)
	g.closers = append(g.closers, func() { g.bus.Close() })

	// Register Eino callbacks → event bus bridge
	cbHandler := agent.NewEventBusHandler(g.bus, events.SourceAgent)
	einoCallbacks.AppendGlobalHandlers(cbHandler)

	// Event persistence
	logsDir := filepath.Join(config.OzziePath(), "logs")
	eventLogger := events.NewEventLogger(logsDir, g.bus)
	g.closers = append(g.closers, func() { eventLogger.Close() })

	// Validate provider capabilities (warning only — allows future extensibility)
	for name, prov := range g.cfg.Models.Providers {
		if len(prov.Capabilities) > 0 {
			if err := models.ValidateCapabilities(prov.Capabilities); err != nil {
				slog.Warn("provider has unknown capability", "provider", name, "error", err)
			}
		}
	}

	return nil
}

// initModels creates the model registry, wires the reloader hook, and
// resolves the default chat model.
func (g *gateway) initModels() error {
	// Model registry (with keyring for encrypted auth)
	g.registry = models.NewRegistry(g.cfg.Models, g.kr)

	// Wire reloader → model registry for hot config reload
	g.reloader.OnReload(func(newCfg *config.Config) {
		g.registry.UpdateProviders(newCfg.Models)
	})

	// Get default model
	var err error
	g.chatModel, err = g.registry.Default(g.ctx)
	if err != nil {
		return fmt.Errorf("init default model: %w", err)
	}
	return nil
}

// initToolPipeline sets up the tool registry, MCP servers, permissions,
// sandbox, constraints, and dangerous tool wrapper.
func (g *gateway) initToolPipeline() error {
	// Plugin registry — load WASM plugins + register native tools
	var err error
	g.toolRegistry, err = hands.SetupToolRegistry(g.ctx, g.cfg, g.bus)
	if err != nil {
		return fmt.Errorf("setup tools: %w", err)
	}
	g.closers = append(g.closers, func() { g.toolRegistry.Close(g.ctx) })

	// MCP servers — connect to external MCP tool servers
	if err := hands.SetupMCPServers(g.ctx, g.cfg.MCP, g.toolRegistry, g.bus); err != nil {
		slog.Warn("failed to setup MCP servers", "error", err)
	}

	// Tool permissions — global auto-approved tools from config
	g.toolPerms = conscience.NewToolPermissions(g.cfg.Tools.AllowedDangerous)

	// Prepare tmp dir early — needed by both sandbox guard and filesystem middleware
	g.tmpDir = filepath.Join(config.OzziePath(), "tmp")
	if err := os.MkdirAll(g.tmpDir, 0o755); err != nil {
		return fmt.Errorf("create tmp dir: %w", err)
	}

	// Sandbox guard — validates command content in autonomous mode (before dangerous wrapper)
	if g.cfg.Sandbox.IsSandboxEnabled() {
		sandboxPaths := append([]string{g.tmpDir}, g.cfg.Sandbox.AllowedPaths...)
		hands.WrapRegistrySandbox(g.toolRegistry, sandboxPaths)
	}

	// Constraint guard — per-tool argument validation (between sandbox and dangerous)
	hands.WrapRegistryConstraints(g.toolRegistry)

	// Wrap dangerous tools with confirmation for interactive gateway
	hands.WrapRegistryDangerous(g.toolRegistry, g.bus, g.toolPerms)

	return nil
}

// initSkills loads skill definitions, creates the verifier, runner config,
// catalog, and executor.
func (g *gateway) initSkills() error {
	// Skill registry — load from SKILL.md directories
	g.skillRegistry = skills.NewRegistry()
	for _, dir := range g.cfg.Skills.Dirs {
		if err := g.skillRegistry.LoadDir(dir); err != nil {
			slog.Warn("failed to load skills", "dir", dir, "error", err)
		}
	}
	slog.Info("skills loaded", "count", len(g.skillRegistry.All()))

	// Verifier for acceptance criteria — uses a SummarizeFunc closure
	verifier := skills.NewVerifier(func(ctx context.Context, prompt string) (string, error) {
		chatModel, err := g.registry.Default(ctx)
		if err != nil {
			return "", err
		}
		resp, err := chatModel.Generate(ctx, []*schema.Message{
			{Role: schema.User, Content: prompt},
		})
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	})

	g.skillRunCfg = skills.RunnerConfig{
		RunnerFactory: agent.NewRunnerFactory(g.registry),
		ToolLookup:    g.toolRegistry.AsDomainToolLookup(),
		EventBus:      g.bus,
		Verifier:      verifier,
	}

	// Catalog for context middleware (name → description only)
	g.skillDescs = g.skillRegistry.Catalog()

	// Skill executor for direct skill tasks
	g.skillExecutor = skills.NewPoolSkillExecutor(g.skillRegistry, g.skillRunCfg)

	return nil
}

// initStores creates session and task stores, runs crash recovery,
// starts the heartbeat writer, and registers the update_session tool.
func (g *gateway) initStores() error {
	// Session store
	sessionsDir := filepath.Join(config.OzziePath(), "sessions")
	g.sessionStore = sessions.NewFileStore(sessionsDir)

	// Task store
	tasksDir := filepath.Join(config.OzziePath(), "tasks")
	g.taskStore = tasks.NewFileStore(tasksDir)

	// Crash recovery — re-queue interrupted tasks
	recovered, recoverErr := tasks.RecoverTasks(g.taskStore)
	if recoverErr != nil {
		slog.Warn("task recovery", "error", recoverErr)
	}
	if recovered > 0 {
		slog.Info("recovered interrupted tasks", "count", recovered)
	}

	// Heartbeat writer
	hbWriter := heartbeat.NewWriter(filepath.Join(config.OzziePath(), "heartbeat.json"))
	hbWriter.Start()
	g.closers = append(g.closers, func() { hbWriter.Stop() })

	// Register update_session tool (needs session store, so registered here)
	updateSessionTool := hands.NewUpdateSessionTool(g.sessionStore)
	if err := g.toolRegistry.RegisterNative("update_session", updateSessionTool, hands.UpdateSessionManifest()); err != nil {
		slog.Warn("failed to register update_session tool", "error", err)
	}

	// Resolve default model tier (used for prompt adaptation)
	g.defaultTier = g.registry.DefaultTier()
	slog.Info("default model tier", "tier", g.defaultTier)

	return nil
}

// initPolicy creates the policy resolver and pairing store.
func (g *gateway) initPolicy() {
	// Convert config overrides to policy.Override
	overrides := make(map[string]policy.Override, len(g.cfg.Policies.Overrides))
	for name, ov := range g.cfg.Policies.Overrides {
		overrides[name] = policy.Override{
			AllowedSkills: ov.AllowedSkills,
			AllowedTools:  ov.AllowedTools,
			DeniedTools:   ov.DeniedTools,
			ApprovalMode:  ov.ApprovalMode,
			ClientFacing:  ov.ClientFacing,
			MaxConcurrent: ov.MaxConcurrent,
		}
	}
	g.policyResolver = policy.NewPolicyResolver(overrides)

	pairingsDir := filepath.Join(config.OzziePath(), "pairings")
	g.pairingStore = policy.NewPairingStore(pairingsDir)

	slog.Info("policy system initialized", "policies", g.policyResolver.Names())
}

// initMemory opens the SQLite memory store, optionally creates the vector
// store and embedding pipeline, wires the reloader hook for hot-reload,
// and starts the cross-task learning extractor.
func (g *gateway) initMemory() error {
	// Memory store (SQLite) + optional vector embedding
	memoryDir := filepath.Join(config.OzziePath(), "memory")
	var err error
	g.memoryStore, err = memory.NewSQLiteStore(memoryDir)
	if err != nil {
		return fmt.Errorf("open memory store: %w", err)
	}
	g.closers = append(g.closers, func() { g.memoryStore.Close() })

	var vectorStore memory.VectorStorer
	if g.cfg.Embedding.IsEnabled() {
		embedder, embedErr := membridge.NewEmbedder(g.ctx, g.cfg.Embedding, g.kr)
		if embedErr != nil {
			slog.Warn("embedding disabled: failed to create embedder", "error", embedErr)
		} else {
			dims := g.cfg.Embedding.Dims
			if dims <= 0 {
				dims = 1536 // default for most models
			}
			vs, vsErr := memory.NewSQLiteVectorStore(g.memoryStore.DB(), embedder, dims)
			if vsErr != nil {
				slog.Warn("embedding disabled: failed to create vector store", "error", vsErr)
			} else {
				vectorStore = vs
				queueSize := g.cfg.Embedding.QueueSize
				if queueSize <= 0 {
					queueSize = 100
				}
				embeddingModel := g.cfg.Embedding.Model
				g.pipeline = memory.NewPipeline(vectorStore, g.memoryStore, embeddingModel, queueSize)
				g.pipeline.Start(g.ctx)
				g.closers = append(g.closers, func() { g.pipeline.Stop() })

				// Async startup reindex (incremental — skips already-indexed entries)
				go func() {
					if _, err := memory.Reindex(g.ctx, g.memoryStore, vectorStore, embeddingModel); err != nil {
						slog.Warn("startup reindex failed", "error", err)
					}
				}()
				slog.Info("semantic memory enabled", "driver", g.cfg.Embedding.Driver, "model", embeddingModel)
			}
		}
	}

	g.memoryRetriever = memory.NewHybridRetriever(g.memoryStore, vectorStore)
	g.closers = append(g.closers, func() { g.memoryRetriever.Close() })

	// Wire reloader → embedding hot-reload
	if g.cfg.Embedding.IsEnabled() {
		g.embFingerprint = membridge.EmbeddingFingerprint(g.cfg.Embedding)
	}
	g.reloader.OnReload(func(newCfg *config.Config) {
		newFP := ""
		if newCfg.Embedding.IsEnabled() {
			newFP = membridge.EmbeddingFingerprint(newCfg.Embedding)
		}
		if newFP == g.embFingerprint {
			return
		}
		oldFP := g.embFingerprint
		g.embFingerprint = newFP

		if g.pipeline == nil {
			// Was disabled at startup — can't hot-enable without restart
			if newFP != "" {
				slog.Warn("embedding config changed but was disabled at startup, restart gateway to apply")
			}
			return
		}

		if !newCfg.Embedding.IsEnabled() {
			g.pipeline.Swap(nil, "")
			g.memoryRetriever.SwapVector(nil)
			slog.Info("embedding disabled via config reload")
			return
		}

		// Recreate embedder + vector store
		newEmbedder, err := membridge.NewEmbedder(g.ctx, newCfg.Embedding, g.kr)
		if err != nil {
			slog.Error("embedding reload: create embedder failed", "error", err)
			return
		}
		newDims := newCfg.Embedding.Dims
		if newDims <= 0 {
			newDims = 1536
		}
		newVS, err := memory.NewSQLiteVectorStore(g.memoryStore.DB(), newEmbedder, newDims)
		if err != nil {
			slog.Error("embedding reload: create vector store failed", "error", err)
			return
		}
		newModel := newCfg.Embedding.Model
		g.pipeline.Swap(newVS, newModel)
		g.memoryRetriever.SwapVector(newVS)

		go func() {
			if _, err := memory.Reindex(g.ctx, g.memoryStore, newVS, newModel); err != nil {
				slog.Warn("embedding reload reindex failed", "error", err)
			}
		}()
		slog.Info("embedding reloaded", "old", oldFP, "new", newFP)
	})

	// Cross-task learning: extract reusable lessons from completed tasks
	if g.pipeline != nil {
		extractor := membridge.NewExtractor(membridge.ExtractorConfig{
			Store:      g.memoryStore,
			Pipeline:   g.pipeline,
			TaskReader: g.taskStore,
			Summarizer: &extractorLLMAdapter{chatModel: g.chatModel},
			Bus:        g.bus,
			Retriever:  g.memoryRetriever,
		})
		extractor.Start()
		g.closers = append(g.closers, func() { extractor.Stop() })
	}

	return nil
}

// initRuntime creates the actor pool, schedule store, skill schedules,
// and the scheduler.
func (g *gateway) initRuntime() error {
	// Actor pool — capacity-aware LLM orchestration (replaces WorkerPool)
	providerSpecs := make(map[string]actors.ProviderSpec, len(g.cfg.Models.Providers))
	for name, prov := range g.cfg.Models.Providers {
		providerSpecs[name] = actors.ProviderSpec{
			MaxConcurrent: prov.MaxConcurrent,
			Tags:          prov.Tags,
			Capabilities:  prov.Capabilities,
			PromptPrefix:  prov.PromptPrefix,
		}
	}
	g.pool = actors.NewActorPool(actors.ActorPoolConfig{
		Providers:       providerSpecs,
		Store:           g.taskStore,
		Bus:             g.bus,
		RunnerFactory:   agent.NewRunnerFactory(g.registry),
		TierResolver:    g.registry,
		ToolLookup:      g.toolRegistry.AsDomainToolLookup(),
		SkillRunner:     g.skillExecutor,
		TaskMiddlewares: g.taskMws,
		Retriever:       g.memoryRetriever,
		Perms:           g.toolPerms,
		ExecutorFactory: tasks.NewTaskExecutorFactory(),
	})
	g.pool.Start()
	g.closers = append(g.closers, func() { g.pool.Stop() })

	// Schedule store — persistent dynamic schedule entries
	schedulesDir := filepath.Join(config.OzziePath(), "schedules")
	scheduleStore := scheduler.NewScheduleStore(schedulesDir)

	// Extract skill schedule info for the scheduler (avoids import cycle)
	var schedSkills []scheduler.SkillScheduleInfo
	for _, sk := range g.skillRegistry.All() {
		if sk.Triggers == nil || !sk.Triggers.HasScheduleTrigger() {
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
	g.sched = scheduler.New(scheduler.Config{
		Pool:   g.pool,
		Bus:    g.bus,
		Skills: schedSkills,
		Store:  scheduleStore,
	})
	g.sched.Start()
	g.closers = append(g.closers, func() { g.sched.Stop() })

	return nil
}

// registerTools registers all native tools (memory, tasks, schedules,
// activation, skills, workflow) and builds the ToolSet.
func (g *gateway) registerTools() {
	// Register memory tools
	storeMemTool := memtools.NewStoreMemoryTool(g.memoryStore, g.pipeline)
	if err := g.toolRegistry.RegisterNative("store_memory", storeMemTool, hands.StoreMemoryManifest()); err != nil {
		slog.Warn("failed to register store_memory tool", "error", err)
	}

	queryMemTool := memtools.NewQueryMemoriesTool(g.memoryRetriever)
	if err := g.toolRegistry.RegisterNative("query_memories", queryMemTool, hands.QueryMemoriesManifest()); err != nil {
		slog.Warn("failed to register query_memories tool", "error", err)
	}

	forgetMemTool := memtools.NewForgetMemoryTool(g.memoryStore, g.pipeline)
	if err := g.toolRegistry.RegisterNative("forget_memory", forgetMemTool, hands.ForgetMemoryManifest()); err != nil {
		slog.Warn("failed to register forget_memory tool", "error", err)
	}

	// Register task tools
	submitTool := hands.NewSubmitTaskTool(g.pool, g.toolRegistry, g.toolPerms, g.bus)
	if err := g.toolRegistry.RegisterNative("submit_task", submitTool, hands.SubmitTaskManifest()); err != nil {
		slog.Warn("failed to register submit_task tool", "error", err)
	}

	checkTool := hands.NewCheckTaskTool(g.taskStore)
	if err := g.toolRegistry.RegisterNative("check_task", checkTool, hands.CheckTaskManifest()); err != nil {
		slog.Warn("failed to register check_task tool", "error", err)
	}

	cancelTool := hands.NewCancelTaskTool(g.pool)
	if err := g.toolRegistry.RegisterNative("cancel_task", cancelTool, hands.CancelTaskManifest()); err != nil {
		slog.Warn("failed to register cancel_task tool", "error", err)
	}

	planTool := hands.NewPlanTaskTool(g.pool)
	if err := g.toolRegistry.RegisterNative("plan_task", planTool, hands.PlanTaskManifest()); err != nil {
		slog.Warn("failed to register plan_task tool", "error", err)
	}

	listTasksTool := hands.NewListTasksTool(g.taskStore)
	if err := g.toolRegistry.RegisterNative("list_tasks", listTasksTool, hands.ListTasksManifest()); err != nil {
		slog.Warn("failed to register list_tasks tool", "error", err)
	}

	// Register schedule tools
	scheduleTaskTool := hands.NewScheduleTaskTool(g.sched, g.bus, g.toolRegistry, g.toolPerms)
	if err := g.toolRegistry.RegisterNative("schedule_task", scheduleTaskTool, hands.ScheduleTaskManifest()); err != nil {
		slog.Warn("failed to register schedule_task tool", "error", err)
	}

	unscheduleTaskTool := hands.NewUnscheduleTaskTool(g.sched, g.bus)
	if err := g.toolRegistry.RegisterNative("unschedule_task", unscheduleTaskTool, hands.UnscheduleTaskManifest()); err != nil {
		slog.Warn("failed to register unschedule_task tool", "error", err)
	}

	listSchedulesTool := hands.NewListSchedulesTool(g.sched)
	if err := g.toolRegistry.RegisterNative("list_schedules", listSchedulesTool, hands.ListSchedulesManifest()); err != nil {
		slog.Warn("failed to register list_schedules tool", "error", err)
	}

	triggerScheduleTool := hands.NewTriggerScheduleTool(g.sched)
	if err := g.toolRegistry.RegisterNative("trigger_schedule", triggerScheduleTool, hands.TriggerScheduleManifest()); err != nil {
		slog.Warn("failed to register trigger_schedule tool", "error", err)
	}

	// Register approve_pairing tool (needs policy resolver + pairing store)
	approvePairingTool := hands.NewApprovePairingTool(g.pairingStore, g.policyResolver, g.bus)
	if err := g.toolRegistry.RegisterNative("approve_pairing", approvePairingTool, hands.ApprovePairingManifest()); err != nil {
		slog.Warn("failed to register approve_pairing tool", "error", err)
	}

	// ToolSet: all native tools are always active (core).
	// Only MCP tools require on-demand activation via activate_tools.
	// Gemini Flash doesn't handle the activate-then-use pattern reliably.
	coreTools := g.toolRegistry.NativeToolNames()
	g.toolSet = brain.NewToolSet(coreTools, g.toolRegistry.ToolNames())

	// Register activate_tools meta-tool (needs toolSet + toolRegistry).
	// Registered AFTER NewToolSet, so we must explicitly add it to core.
	activateTool := hands.NewActivateToolsTool(g.toolSet, g.toolRegistry)
	if err := g.toolRegistry.RegisterNative("activate_tools", activateTool, hands.ActivateToolsManifest()); err != nil {
		slog.Warn("failed to register activate_tools tool", "error", err)
	}
	g.toolSet.RegisterCore("activate_tools")

	// Register activate_skill (core tool — always active, progressive disclosure)
	activateSkillTool := hands.NewActivateSkillTool(g.skillRegistry, g.toolSet, g.toolRegistry)
	if err := g.toolRegistry.RegisterNative("activate_skill", activateSkillTool, hands.ActivateSkillManifest()); err != nil {
		slog.Warn("failed to register activate_skill tool", "error", err)
	}
	g.toolSet.RegisterCore("activate_skill")

	// Register run_workflow (on-demand — activated via activate_skill or explicitly)
	runWorkflowTool := hands.NewRunWorkflowTool(g.skillExecutor)
	if err := g.toolRegistry.RegisterNative("run_workflow", runWorkflowTool, hands.RunWorkflowManifest()); err != nil {
		slog.Warn("failed to register run_workflow tool", "error", err)
	}

	slog.Info("tools loaded", "count", len(g.toolRegistry.ToolNames()))
}

// initAgent builds the persona, middlewares (filesystem, reduction, context,
// sub-agent), the AgentFactory, cost tracker, layered context, and event
// runner.
func (g *gateway) initAgent() error {
	// Agent — persona from SOUL.md or DefaultPersona fallback (layer 1)
	g.persona = agent.PersonaForTier(agent.LoadPersona(), g.defaultTier)
	slog.Debug("loaded persona", "length", len(g.persona), "persona", g.persona)

	// Filesystem middleware — provides ls, read_file, write_file, edit_file, glob, grep via Eino ADK
	fsOpts := []agent.OzzieBackendOption{
		agent.WithWriteAllowedPaths(g.tmpDir),
		agent.WithReadRestrictedPaths(secrets.AgeDirPath()),
	}
	fsBackend := agent.NewOzzieBackend(g.bus, g.toolPerms, fsOpts...)

	// Register str_replace_editor (filesystem-based, needs fsBackend)
	hands.RegisterFilesystemTools(g.toolRegistry, fsBackend)
	g.toolSet.RegisterCore("str_replace_editor")

	fsMw, err := einoFs.NewMiddleware(g.ctx, &einoFs.Config{
		Backend:                          fsBackend,
		WithoutLargeToolResultOffloading: true, // offloading handled by reduction middleware below
	})
	if err != nil {
		return fmt.Errorf("init filesystem middleware: %w", err)
	}

	// Reduction middleware — clears old tool results and offloads large ones to filesystem
	reductionMw, err := einoReduction.NewToolResultMiddleware(g.ctx, &einoReduction.ToolResultConfig{
		Backend:                fsBackend,
		ClearingTokenThreshold: 20000,
		KeepRecentTokens:       40000,
		OffloadingTokenLimit:   20000,
	})
	if err != nil {
		return fmt.Errorf("init reduction middleware: %w", err)
	}

	// Build runtime instruction (system tools + environment awareness)
	systemTools := prompt.LoadSystemTools(g.cfg.Runtime.SystemToolsFile)
	runtimeInstruction := prompt.RuntimeSection(g.cfg.Runtime.Environment, systemTools)

	// SubAgent middleware — injects SubAgentInstructions + runtime (tool reference + workflow)
	subAgentMw := agent.NewSubAgentMiddleware(runtimeInstruction, g.defaultTier)

	// Task middlewares — subagent instructions + filesystem + reduction for sub-agents (no context middleware)
	// Stored as []any (opaque) — the RunnerFactory adapter casts them back to adk.AgentMiddleware.
	g.taskMws = []any{subAgentMw, fsMw, reductionMw}

	// Build full tool descriptions for prompt composer
	allToolDescs := g.toolRegistry.AllToolDescriptions()

	// Build actor descriptions for the planner prompt (non-default providers only)
	var actorDescs []prompt.ActorInfo
	for name, prov := range g.cfg.Models.Providers {
		if name == g.cfg.Models.Default {
			continue // skip the default provider — it's the planner itself
		}
		actorDescs = append(actorDescs, prompt.ActorInfo{
			Name:         name,
			Tags:         prov.Tags,
			Capabilities: prov.Capabilities,
			PromptPrefix: prov.PromptPrefix,
		})
	}

	// Context middleware — injects dynamic context (instructions, tools, session, memories)
	contextMw := agent.NewContextMiddleware(agent.ContextMiddlewareConfig{
		CustomInstructions:  g.cfg.Agent.SystemPrompt,
		PreferredLanguage:   g.cfg.Agent.PreferredLanguage,
		RuntimeInstruction:  runtimeInstruction,
		AllToolDescriptions: allToolDescs,
		SkillDescriptions:   g.skillDescs,
		Store:               g.sessionStore,
		ToolSet:             g.toolSet,
		Retriever:           g.memoryRetriever,
		Tier:                g.defaultTier,
		ActorDescriptions:   actorDescs,
	})

	var middlewares []adk.AgentMiddleware
	middlewares = append(middlewares, fsMw, reductionMw, contextMw)

	// AgentFactory (replaces single runner — creates fresh runner per turn)
	g.factory = agent.NewAgentFactory(g.chatModel, g.persona, middlewares)

	// Cost tracker — accumulates token usage per session
	sessionsDir := filepath.Join(config.OzziePath(), "sessions")
	costTracker := sessions.NewCostTracker(g.bus, g.sessionStore)
	g.closers = append(g.closers, func() { costTracker.Close() })

	// Layered context manager (optional)
	if g.cfg.LayeredContext.IsEnabled() {
		layeredStore := layeredctx.NewStore(sessionsDir)
		layeredCfg := layeredctx.DefaultConfig()
		layeredCfg.Enabled = true
		layeredCfg.MaxArchives = g.cfg.LayeredContext.MaxArchives
		layeredCfg.MaxRecentMessages = g.cfg.LayeredContext.MaxRecentMessages
		layeredCfg.ArchiveChunkSize = g.cfg.LayeredContext.ArchiveChunkSize
		layeredCfg.MaxPromptTokens = g.registry.DefaultContextWindow()
		chatModel := g.chatModel
		g.layered = layeredctx.NewManager(layeredStore, layeredCfg,
			func(ctx context.Context, prompt string) (string, error) {
				resp, err := chatModel.Generate(ctx, []*schema.Message{{Role: schema.User, Content: prompt}})
				if err != nil {
					return "", err
				}
				return resp.Content, nil
			},
		)
		slog.Info("layered context enabled",
			"max_archives", layeredCfg.MaxArchives,
			"max_recent", layeredCfg.MaxRecentMessages,
		)
	}

	// Event runner with dynamic tool selection and actor pool integration
	g.eventRunner = agent.NewEventRunner(agent.EventRunnerConfig{
		Factory:         g.factory,
		ToolSet:         g.toolSet,
		Registry:        g.toolRegistry,
		EventBus:        g.bus,
		Store:           g.sessionStore,
		Pool:            actors.NewPoolAdapter(g.pool),
		DefaultProvider: g.registry.DefaultName(),
		ContextWindow:   g.registry.DefaultContextWindow(),
		Tier:            g.defaultTier,
		Layered:         g.layered,
	})
	g.closers = append(g.closers, func() { g.eventRunner.Close() })

	return nil
}

// initConnectors creates and starts external platform connectors (Discord, etc.).
// Must be called after initAgent (EventRunner must be ready).
func (g *gateway) initConnectors() {
	var conns []connector.Connector

	// Discord connector
	if dc := g.cfg.Connectors.Discord; dc != nil && dc.Token != "" {
		discordConn, err := discord.New(discord.Config{
			Token:        dc.Token,
			AdminChannel: dc.AdminChannel,
		})
		if err != nil {
			slog.Error("failed to create discord connector", "error", err)
		} else {
			conns = append(conns, discordConn)

			// Pairing notifier → admin channel
			pn := eyes.NewPairingNotifier(g.bus, discordConn)
			g.closers = append(g.closers, func() { pn.Stop() })
		}
	}

	if len(conns) == 0 {
		return
	}

	// Session map
	sessionMapDir := filepath.Join(config.OzziePath(), "connectors")
	sessionMap := eyes.NewSessionMap(sessionMapDir)

	// Manager
	g.connectorManager = eyes.NewManager(eyes.ManagerConfig{
		Bus:          g.bus,
		SessionStore: g.sessionStore,
		PairingStore: g.pairingStore,
		SessionMap:   sessionMap,
		Connectors:   conns,
	})
	g.connectorManager.Start()
	g.closers = append(g.closers, func() { g.connectorManager.Stop() })

	// Progress reactor — emoji reactions on connector messages
	pr := eyes.NewProgressReactor(eyes.ProgressReactorConfig{
		Bus:        g.bus,
		Connectors: g.connectorManager.ConnectorsByName(),
		SessionMap: sessionMap,
	})
	g.closers = append(g.closers, func() { pr.Stop() })

	slog.Info("connectors started", "count", len(conns))
}
