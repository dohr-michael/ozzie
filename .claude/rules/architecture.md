---
description: Ozzie architecture rules and conventions
globs: "**/*.go"
---

# Architecture

Ozzie follows the **gateway pattern**: one persistent process (`ozzie gateway`) orchestrates everything, clients connect via WebSocket.

## Package layout

```
cmd/ozzie/                             → entry point
cmd/commands/                          → CLI commands (urfave/cli v3)
internal/config/                       → JSONC config loader (shared, neither core nor infra)

# ── core/ = pure domain (zero Eino dependency) ──
internal/core/brain/                   → domain ports (Tool, Runner, Message, ToolSet, ModelTier)
internal/core/events/                  → event bus + event logger
internal/core/prompt/                  → prompt template registry, auditable composer, section builders
internal/core/actors/                  → capacity-aware actor pool
internal/core/layered/                 → L0/L1/L2 context compression
internal/core/skills/                  → DAG workflow engine
internal/core/conscience/              → dangerous tool wrapper, sandbox, permissions, constraints
internal/core/introspection/           → logger setup, log level resolution
internal/core/policy/                  → pairing policy

# ── infra/ = adapters (Eino / infrastructure) ──
internal/infra/agent/                  → Eino ADK adapter (EventRunner, AgentFactory, callbacks, middlewares)
internal/infra/models/                 → LLM model registry (Anthropic, OpenAI, Gemini, Mistral, Ollama via Eino)
internal/infra/membridge/              → memory wiring adapter (embedder factory, cross-task extractor)
internal/infra/hands/                  → native tools, WASM plugins, MCP client, tool registry
internal/infra/gateway/                → HTTP server (chi) + WebSocket hub
  internal/infra/gateway/ws/           → WebSocket hub
internal/infra/sessions/               → session store + cost tracker
internal/infra/tasks/                  → async task runner + file store
internal/infra/scheduler/              → cron + event-based scheduler
internal/infra/eyes/                   → connector wiring + manager
  internal/infra/eyes/discord/         → Discord connector implementation
internal/infra/mcp/                    → MCP server (expose tools via MCP)
internal/infra/storage/dirstore/       → generic directory-based persistence
internal/infra/secrets/                → secret management (age encryption, dotenv)
internal/infra/auth/                   → authentication middleware
internal/infra/heartbeat/              → heartbeat monitoring
internal/infra/i18n/                   → internationalization
internal/infra/ui/                     → UI components + setup wizard

# ── pkg/ = importable libraries ──
pkg/names/                             → friendly ID generation (SF-themed, importable)
pkg/memory/                            → semantic memory library (hybrid keyword + vector, importable)
pkg/memory/tools/                      → memory tools (store, query, forget — Eino InvokableTool)
pkg/llmutil/                           → LLM response utilities (code fence stripping)
pkg/connector/                         → connector interfaces (importable)
pkg/editor/                            → editor integration
pkg/htmltext/                          → HTML text utilities

clients/ws/                            → WS client library
clients/tui/                           → TUI client (Bubbletea)
```

## OZZIE_PATH

All Ozzie data lives under a single root directory:
- `$OZZIE_PATH` if set, otherwise `~/.ozzie`
- Created by `ozzie wake` (onboarding command)
- Contains: `config.jsonc`, `.env`, `logs/`, `skills/`, `sessions/`
- Resolved via `config.OzziePath()`, `config.ConfigPath()`, `config.DotenvPath()`

## Key conventions

- **Event-driven**: components communicate through the event bus, not direct calls
- **Config**: JSONC with `${{ .Env.VAR }}` templating. Defaults are applied in `loader.go`
- **Models**: lazy-initialized via Registry. 5 drivers: anthropic, openai, gemini, mistral, ollama. Auth resolved from config → env var → driver default. Ollama needs no auth (local). Gemini uses `google.golang.org/genai` SDK
- **DDD Hexagonal**: `core/` is a pure domain layer (zero Eino imports). Domain ports (`brain.Tool`, `brain.Runner`, `brain.RunnerFactory`, `brain.ToolLookup`) are defined in `core/brain/ports.go`. Eino-specific adapters live in `infra/agent/`. Conversion happens at adapter boundaries. **`core/` must never import `infra/`**.
- **ToolSet**: two-tier — ~18 core tools always active, plugin/MCP tools activated on demand via `activate`. Unified tools: `activate` (tools + skills), `query_tasks` (check + list), `submit_task` (single + multi-step plan), `web` (fetch + search). Eino ADK freezes tools per-run, so `AgentFactory` creates a fresh runner per turn. `EventRunner` retries after mid-turn activation (buffered → streaming)
- **Dangerous tools**: `DangerousToolWrapper` prompts with 3 options (allow once / always for session / deny). Approvals persisted in `Session.ApprovedTools`. `trusted_tools` in MCP config bypasses confirmation. Pre-approval in `submit_task` and schedules
- **MCP client**: external MCP servers in `config.mcp.servers`. Tools are dangerous by default. `allowed_tools` / `denied_tools` / `trusted_tools` for fine-grained control
- **CLI v3**: `ActionFunc` is `func(context.Context, *Command) error` — no `*cli.Context`
- **WebSocket**: uses `github.com/coder/websocket` (NOT nhooyr.io which is deprecated)
- **Eino ADK**: `ChatModelAgentConfig` requires `Name`, `Description`, uses `Instruction` (not SystemPrompt). `NewRunner` takes `RunnerConfig` struct. `Runner.Run` takes `[]adk.Message` (alias for `[]*schema.Message`)
