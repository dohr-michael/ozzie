# Ozzie — Agent OS

Personal AI agent operating system. Go 1.25, gateway pattern, event-driven.

## Quality gates

Every change must pass **all three** — no exceptions:

```bash
go build ./...              # compile
~/go/bin/staticcheck ./...  # lint (SA1019, SA4009, etc.)
go test ./...               # tests
```

## Rules

Detailed rules live in `.claude/rules/`:

- **go-quality.md** — build/lint/test gates, Go style
- **architecture.md** — package layout, key conventions, API gotchas (Eino ADK, cli/v3, coder/websocket)
- **language.md** — French conversations, English code

## Current state

Full agent OS with 5 LLM drivers, coordinator pattern, async tasks, semantic memory,
WASM plugins, MCP client/server, skill engine, scheduler, and dangerous tool approval flow.

Working E2E: `ozzie gateway` → `ozzie ask "hello"` → streamed LLM response with tool calling.

## Key files

| What              | Where                                                   |
|-------------------|---------------------------------------------------------|
| Entry point       | `cmd/ozzie/main.go`                                     |
| CLI commands      | `cmd/commands/`                                         |
| Config            | `internal/config/` + `configs/config.example.jsonc`     |
| Event bus         | `internal/events/`                                      |
| Models            | `internal/models/` (registry, anthropic, openai, gemini, mistral, ollama) |
| Gateway           | `internal/gateway/` (chi server + WS hub)               |
| Agent             | `internal/agent/` (Eino ADK + EventRunner + AgentFactory) |
| Tool system       | `internal/plugins/` (native tools, WASM, MCP, dangerous wrapper, sandbox) |
| Tasks             | `internal/tasks/` (runner, pool) + `internal/actors/` (capacity pool) |
| Memory            | `internal/memory/` (hybrid keyword + vector retrieval)  |
| Skills            | `internal/skills/` (DAG workflows)                      |
| Scheduler         | `internal/scheduler/` (cron, event triggers)            |
| Sessions          | `internal/sessions/` (store, approved tools)            |
| Context           | `internal/agent/middleware_context.go` (dynamic prompt composition) |
| Layered context   | `internal/layered/` (L0/L1/L2 compression)             |
| WS client         | `clients/ws/`                                           |
| TUI               | `clients/tui/`                                          |
| Default persona   | `internal/agent/agent.go` → `DefaultPersona` + `AgentInstructions` |

## Key concepts

- **ToolSet** — Two-tier tool system: core tools always active, plugin/MCP tools activated on demand via `activate_tools`. Eino ADK freezes tools per-run → `AgentFactory` creates a fresh runner per turn; `EventRunner` retries after activation.
- **Dangerous tool approval** — `DangerousToolWrapper` prompts user with 3 options (allow once / always for session / deny). Approvals persisted in `Session.ApprovedTools`, restored on reconnect. Pre-approval via `submit_task` and schedules.
- **MCP servers** — External MCP servers configured in `config.mcp.servers`. Tools are `dangerous: true` by default. `trusted_tools` bypasses confirmation for specific tools. `allowed_tools` / `denied_tools` for whitelisting/blacklisting.
- **Model drivers** — 5 drivers: `anthropic`, `openai`, `gemini`, `mistral`, `ollama`. Lazy-init via `Registry`. Auth resolution: config → env var → driver default. Gemini uses `google.golang.org/genai` SDK.
