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
| Domain ports      | `internal/core/brain/` (Tool, Runner, Message, ToolSet, ModelTier) |
| Event bus         | `internal/core/events/`                                 |
| Prompt system     | `internal/core/prompt/` (template registry, composer, section builders) |
| Actors            | `internal/core/actors/` (capacity-aware actor pool)     |
| Conscience        | `internal/core/conscience/` (dangerous wrapper, sandbox, permissions, constraints) |
| Introspection     | `internal/core/introspection/` (logger setup, log level resolution) |
| Layered context   | `internal/core/layered/` (L0/L1/L2 compression)        |
| Skills            | `internal/core/skills/` (DAG workflows)                 |
| Policy            | `internal/core/policy/` (pairing policy)                |
| Agent (adapter)   | `internal/infra/agent/` (Eino ADK adapter — EventRunner, AgentFactory, tool/runner adapters) |
| Models            | `internal/infra/models/` (registry, anthropic, openai, gemini, mistral, ollama) |
| Tool system       | `internal/infra/hands/` (native tools, WASM, MCP)       |
| Memory bridge     | `internal/infra/membridge/` (embedder factory, cross-task extractor) |
| Gateway           | `internal/infra/gateway/` (chi server + WS hub)         |
| Sessions          | `internal/infra/sessions/` (store, approved tools)      |
| Tasks             | `internal/infra/tasks/` (runner, file store)            |
| Scheduler         | `internal/infra/scheduler/` (cron, event triggers)      |
| Connectors        | `internal/infra/eyes/` (connector manager, Discord)     |
| MCP server        | `internal/infra/mcp/`                                   |
| Names             | `pkg/names/` (friendly ID generation, SF-themed)        |
| Memory (lib)      | `pkg/memory/` (SQLite + FTS5 + brute-force cosine, hybrid retrieval, consolidation) |
| Memory tools      | `pkg/memory/tools/` (store, query, forget — Eino InvokableTool) |
| LLM utilities     | `pkg/llmutil/` (code fence stripping)                   |
| Context           | `internal/infra/agent/middleware_context.go` (dynamic prompt composition via `prompt.Composer`) |
| WS client         | `clients/ws/`                                           |
| TUI               | `clients/tui/`                                          |
| Prompt templates  | `internal/core/prompt/catalog.go` → all prompt constants + `DefaultRegistry` |

## Key concepts

- **ToolSet** — Two-tier tool system: core tools always active, plugin/MCP tools activated on demand via `activate_tools`. Eino ADK freezes tools per-run → `AgentFactory` creates a fresh runner per turn; `EventRunner` retries after activation.
- **Dangerous tool approval** — `DangerousToolWrapper` prompts user with 3 options (allow once / always for session / deny). Approvals persisted in `Session.ApprovedTools`, restored on reconnect. Pre-approval via `submit_task` and schedules.
- **MCP servers** — External MCP servers configured in `config.mcp.servers`. Tools are `dangerous: true` by default. `trusted_tools` bypasses confirmation for specific tools. `allowed_tools` / `denied_tools` for whitelisting/blacklisting.
- **Model drivers** — 5 drivers: `anthropic`, `openai`, `gemini`, `mistral`, `ollama`. Lazy-init via `Registry`. Auth resolution: config → env var → driver default. Gemini uses `google.golang.org/genai` SDK.
- **Entity IDs** — Human-readable IDs via `pkg/names`: `sess_cosmic_asimov`, `task_stellar_deckard`. The name **is** the ID (no separate hex UUID). `names.GenerateID(prefix, exists)` guarantees uniqueness with `_XXXX` counter suffix. `names.DisplayName(id)` extracts the readable part. SF-themed: ~50 adjectives × ~200 nouns (~10k base combinations).
- **Memory** — SQLite (modernc.org/sqlite, pure Go) with FTS5 full-text search + brute-force cosine similarity. Markdown files synced as read-only mirror. Multi-level decay (core/important/normal/ephemeral). LLM-based consolidation. Library in `pkg/memory/` (importable), wiring in `internal/infra/membridge/`.
- **Prompt system** — `internal/core/prompt/` centralizes all prompt templates in a `Registry` with named IDs. `Composer` assembles sections and logs a structured manifest (`slog.Debug`) at each composition. Section builders (`ToolSection`, `SessionSection`, etc.) are pure functions with zero internal dependencies.
- **Sandbox** — AST-based command validation via `mvdan.cc/sh/v3`. Declarative denylist replaces regex patterns. Covers ~90% of known bypasses.
