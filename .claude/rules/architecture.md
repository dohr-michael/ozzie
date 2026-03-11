---
description: Ozzie architecture rules and conventions
globs: "**/*.go"
---

# Architecture

Ozzie follows the **gateway pattern**: one persistent process (`ozzie gateway`) orchestrates everything, clients connect via WebSocket.

## Package layout

```
cmd/ozzie/                    → entry point
cmd/commands/                 → CLI commands (urfave/cli v3)
internal/config/              → JSONC config loader
internal/events/              → event bus + event logger
internal/models/              → LLM model registry (Anthropic, OpenAI, Gemini, Mistral, Ollama via Eino)
internal/gateway/             → HTTP server (chi) + WebSocket hub
internal/agent/               → Eino ADK agent + EventRunner + AgentFactory + ToolSet + callbacks bridge
internal/prompt/              → prompt template registry, auditable composer, section builders
internal/plugins/             → native tools, WASM plugins, MCP client, dangerous wrapper, sandbox
internal/tasks/               → async task runner + pool
internal/actors/              → capacity-aware actor pool
internal/connectors/          → connector wiring + manager
internal/connectors/discord/  → Discord connector implementation
internal/memory/bridge/       → memory wiring (embedder factory, cross-task extractor)
internal/memory/layered/      → L0/L1/L2 context compression
internal/sessions/            → session store + cost tracker
internal/skills/              → DAG workflow engine
internal/scheduler/           → cron + event-based scheduler
internal/storage/dirstore/    → generic directory-based persistence
internal/mcp/                 → MCP client (external server connections)
pkg/names/                    → friendly ID generation (SF-themed, importable)
pkg/memory/                   → semantic memory library (hybrid keyword + vector, importable)
pkg/memory/tools/             → memory tools (store, query, forget — Eino InvokableTool)
pkg/connector/                → connector interfaces (importable)
clients/ws/                   → WS client library
clients/tui/                  → TUI client (Bubbletea)
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
- **ToolSet**: two-tier — core tools always active, plugin/MCP tools activated on demand via `activate_tools`. Eino ADK freezes tools per-run, so `AgentFactory` creates a fresh runner per turn. `EventRunner` retries after mid-turn activation (buffered → streaming)
- **Dangerous tools**: `DangerousToolWrapper` prompts with 3 options (allow once / always for session / deny). Approvals persisted in `Session.ApprovedTools`. `trusted_tools` in MCP config bypasses confirmation. Pre-approval in `submit_task` and schedules
- **MCP client**: external MCP servers in `config.mcp.servers`. Tools are dangerous by default. `allowed_tools` / `denied_tools` / `trusted_tools` for fine-grained control
- **CLI v3**: `ActionFunc` is `func(context.Context, *Command) error` — no `*cli.Context`
- **WebSocket**: uses `github.com/coder/websocket` (NOT nhooyr.io which is deprecated)
- **Eino ADK**: `ChatModelAgentConfig` requires `Name`, `Description`, uses `Instruction` (not SystemPrompt). `NewRunner` takes `RunnerConfig` struct. `Runner.Run` takes `[]adk.Message` (alias for `[]*schema.Message`)
