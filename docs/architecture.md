# Ozzie Architecture

## Overview

Ozzie is a personal AI agent operating system built in Go. It follows the
**gateway pattern**: a single persistent process (`ozzie gateway`) orchestrates
all components, and clients connect via WebSocket.

The system is **event-driven** — components communicate through a typed event
bus rather than direct function calls. This decouples producers (WebSocket hub,
agent, skills) from consumers (logging, persistence, UI streaming).

```
┌──────────────────────────────────────────────────┐
│                  ozzie gateway                    │
│                                                   │
│  ┌─────────┐   ┌──────────┐   ┌──────────────┐  │
│  │ HTTP/WS │──▶│ Event Bus│◀──│ EventRunner  │  │
│  │  Hub    │   │          │   │  (Agent/ADK) │  │
│  └─────────┘   └────┬─────┘   └──────────────┘  │
│       ▲              │              ▲             │
│       │         ┌────▼─────┐        │             │
│       │         │ Ring     │   ┌────┴─────┐      │
│       │         │ Buffer   │   │  LLM     │      │
│       │         └──────────┘   │ Registry │      │
│       │                        └──────────┘      │
│  ┌────┴────────────────────────────────────┐     │
│  │  Plugins / Skills / Tool Registry       │     │
│  └─────────────────────────────────────────┘     │
└──────────────────────────────────────────────────┘
        ▲                              ▲
        │ WebSocket                    │ Anthropic / OpenAI / Ollama
   ┌────┴────┐
   │ Clients │  (ozzie ask, TUI, etc.)
   └─────────┘
```

## Package Map

| Package | Responsibility | Internal Dependencies |
|---|---|---|
| `cmd/ozzie/` | Entry point, CLI bootstrap | `cmd/commands` |
| `cmd/commands/` | CLI commands (urfave/cli v3) | `config`, `events`, `gateway`, `models`, `agent`, `plugins`, `skills`, `sessions` |
| `internal/config/` | JSONC config loader, env templating, defaults | — |
| `internal/events/` | Event bus, typed payloads, ring buffer, resume tokens | — |
| `internal/models/` | LLM model registry (Anthropic, OpenAI, Ollama via Eino) | `config` |
| `internal/auth/` | Authentication (local token, middleware) | `secrets`, `config` |
| `internal/gateway/` | HTTP server (chi), routes | `events`, `gateway/ws`, `sessions`, `auth` |
| `internal/gateway/ws/` | WebSocket hub, client management, protocol frames | `events`, `sessions` |
| `internal/agent/` | Eino ADK agent, EventRunner, prompt composition | `events`, `sessions`, `config` |
| `internal/callbacks/` | Eino callbacks → event bus bridge | `events` |
| `internal/plugins/` | Plugin system (WASM + native), manifest, tool registry, AST sandbox | — |
| `internal/skills/` | Skill system (simple + workflow DAG), skill-as-tool | `agent`, `events`, `models`, `plugins` |
| `pkg/memory/` | Semantic memory library (SQLite + FTS5 + brute-force cosine, hybrid retrieval, consolidation) | — |
| `pkg/memory/tools/` | Memory tools (store, query, forget — Eino InvokableTool) | `pkg/memory` |
| `internal/membridge/` | Memory wiring (embedder factory, cross-task extractor) | `pkg/memory`, `config`, `events`, `models` |
| `internal/sessions/` | Session metadata + JSONL message persistence | — |
| `internal/storage/` | Low-level storage abstractions | — |
| `internal/mcp/` | Model Context Protocol client | — |
| `internal/scheduler/` | Scheduled task triggers (stub) | — |
| `clients/ws/` | WebSocket client library | `gateway/ws` |
| `clients/tui/` | Terminal UI (stub) | — |

## Data Flow

A complete request lifecycle from user input to streamed response:

```
User types "hello"
      │
      ▼
┌─────────────┐  WebSocket frame (req)
│  WS Client  │──────────────────────────┐
└─────────────┘                          │
                                         ▼
                               ┌─────────────────┐
                               │    WS Hub        │
                               │  handleRequest() │
                               └────────┬────────┘
                                        │ Publish(EventUserMessage)
                                        ▼
                               ┌─────────────────┐
                               │    Event Bus     │
                               │  (in-memory ch)  │
                               └────────┬────────┘
                                        │ Dispatch to subscribers
                                        ▼
                               ┌─────────────────┐
                               │  EventRunner     │
                               │  processMessage()│
                               └────────┬────────┘
                                        │ Load history, compose prompt
                                        │ Call ADK Runner.Run()
                                        ▼
                               ┌─────────────────┐
                               │  Eino ADK Agent  │
                               │  (ReAct loop)    │
                               └────────┬────────┘
                                        │ LLM API call
                                        ▼
                               ┌─────────────────┐
                               │  LLM Provider    │
                               │  (Anthropic/etc) │
                               └────────┬────────┘
                                        │ Streamed response
                                        ▼
                               ┌─────────────────┐
                               │  Callbacks       │
                               │  → Event Bus     │──▶ EventAssistantStream
                               └─────────────────┘          │
                                                             ▼
                                                    ┌────────────────┐
                                                    │    WS Hub      │
                                                    │  (subscriber)  │
                                                    └───────┬────────┘
                                                            │ WebSocket frame (event)
                                                            ▼
                                                    ┌────────────────┐
                                                    │   WS Client    │
                                                    └────────────────┘
```

## Event System

### Event Types

Events are organized by domain:

| Category | Events |
|---|---|
| **Boundaries** | `incoming.message`, `outgoing.message` |
| **User/Agent** | `user.message`, `assistant.stream`, `assistant.message` |
| **Tools** | `tool.call`, `tool.call.confirmation` |
| **Prompts** | `prompt.request`, `prompt.response` |
| **Internal** | `internal.llm.call` |
| **Sessions** | `session.created`, `session.closed` |
| **Skills** | `skill.started`, `skill.completed`, `skill.step.started`, `skill.step.completed` |
| **Scheduler** | `schedule.trigger` |

### Bus

The event bus (`events.Bus`) is an in-memory channel-based dispatcher:

- **Publish**: non-blocking send to a buffered channel
- **Subscribe**: register handler for specific event types (or all)
- **SubscribeChan**: channel-based subscription for select loops
- **History**: ring buffer stores recent events for the `/api/events` endpoint (supports `?session=...` and `?type=...` filters)
- **Typed payloads**: `NewTypedEvent` creates events from Go structs; `ExtractPayload[T]` does generic extraction

### Sources

Each event carries a `Source` identifying the emitter: `agent`, `hub`, `ws`, `plugin`, `skill`.

## Plugin System

Plugins extend Ozzie with new tools. Two runtimes are supported:

1. **Extism (WASM)**: sandboxed plugins loaded from `.wasm` files
2. **Native**: Go command plugins compiled and executed as subprocesses

### Manifest

Each plugin declares a JSONC manifest (`manifest.jsonc`):

```jsonc
{
  "name": "weather",
  "description": "Get weather forecasts",
  "level": "tool",           // "tool" or "communication"
  "provider": "extism",      // "extism" or "native"
  "wasm_path": "weather.wasm",
  "capabilities": { "http": { "allowed_hosts": ["api.weather.com"] } },
  "tools": [{
    "name": "get_weather",
    "description": "Get current weather",
    "parameters": {
      "city": { "type": "string", "description": "City name", "required": true }
    }
  }]
}
```

### Capabilities

Plugins request capabilities (`http`, `fs_read`, `fs_write`, `env`, `config`)
that are granted or denied by the host. WASM plugins are sandboxed by default.

### Tool Registry

The `ToolRegistry` aggregates tools from all sources (plugins, skills, MCP)
into a single lookup. Each tool implements Eino's `tool.InvokableTool` interface.

## Skill System

Skills are declarative agent behaviors defined as directories containing a `SKILL.md`
file (instructions in markdown with YAML frontmatter) and an optional `workflow.yaml`
for DAG-based execution.

### SKILL.md Format

Each skill is a directory with a `SKILL.md` file:

```markdown
---
name: code-review
description: Review code for quality issues
allowed-tools:
  - read_file
  - search
---
# Code Review

Review the provided code for quality issues, security vulnerabilities,
and adherence to project conventions.
```

The YAML frontmatter declares metadata and allowed tools. The markdown body
contains the skill instructions loaded as the agent's system prompt.

### Workflow Skills

Skills can include a `workflow.yaml` defining a DAG (directed acyclic graph) of steps:

```yaml
vars:
  env:
    description: Target environment
    required: true

steps:
  - id: build
    title: Build
    instruction: Build the project.
    tools: [run_command]

  - id: test
    title: Test
    instruction: Run tests.
    tools: [run_command]
    needs: [build]

  - id: deploy
    title: Deploy
    instruction: Deploy to {{env}}.
    tools: [run_command]
    needs: [test]
```

Steps execute in parallel where the dependency graph allows. The `WorkflowRunner`
uses Kahn's algorithm for topological ordering and `ReadySteps()` to determine
which steps can run concurrently.

### Skill Activation

The main agent loads skills on demand via `activate_skill`. Once activated, the
skill's instructions and tools become available for the current turn. Skills can
also be used in async tasks via the `skill` parameter in `submit_task`.

## Session & Storage

### Sessions

Each conversation is a `Session` with metadata:
- ID, title, status (active/closed), created/updated timestamps
- Message count, cumulative token usage (input/output)

### Persistence

Sessions are stored on disk using `FileStore`:
- `$OZZIE_PATH/sessions/<session_id>/meta.json` — session metadata
- `$OZZIE_PATH/sessions/<session_id>/messages.jsonl` — message history (append-only)

Token usage is tracked per-session via LLM callback events.

## Prompt Composition

The system prompt is composed from 5 layers:

| Layer | Source | Description |
|---|---|---|
| 1 | `SOUL.md` or `DefaultSystemPrompt` | Core persona (Ozzie character) |
| 2 | `config.Agent.SystemPrompt` | Custom user instructions |
| 3 | Tool Registry + Skill Registry | Active tools and skills manifest |
| 4 | Session store | Session context (title, message count) |
| 5 | Task instructions | Per-turn task context (future) |

Layer 1 is set as the ADK `Instruction`. Layers 2-5 are dynamically composed
by `PromptComposer` and prepended as a system message before the conversation
history.

## Memory System

Ozzie's long-term memory lives in `pkg/memory/` as an importable library. It uses
**SQLite** as the single source of truth, with two query backends:

```
┌──────────────────────────────────────────┐
│       pkg/memory/  (importable lib)      │
│                                          │
│  SQLiteStore (memory.db)                 │
│  ┌─────────┐  ┌──────────┐  ┌────────┐  │
│  │  CRUD   │  │  FTS5    │  │ vec0   │  │
│  │ (SQL)   │  │ (search) │  │(vector)│  │
│  └─────────┘  └──────────┘  └────────┘  │
│                                          │
│  pkg/memory/tools/                       │
│  ┌────────┐  ┌───────┐  ┌────────┐      │
│  │ store  │  │ query │  │ forget │      │
│  └────────┘  └───────┘  └────────┘      │
└──────────┬───────────────────────────────┘
           │  markdown sync
           ▼
    entries/<id>.md  (read-only mirror)

  internal/membridge/
  ├── embedder.go       (factory: config → Eino Embedder)
  └── extractor.go      (cross-task memory extraction)
```

### Storage

- **SQLite** (`memory.db`) via `modernc.org/sqlite` (pure Go, no CGo) with WAL mode,
  FTS5 for full-text search, brute-force cosine similarity for vectors
- **Markdown files** (`entries/<id>.md`) are written on every Create/Update/Delete
  as a human-readable mirror — SQLite is authoritative
- **Hybrid retrieval**: `HybridRetriever` combines FTS5 keyword search (30%) with
  cosine similarity vector search (70%) when embeddings are enabled

### Multi-level Decay

Each memory has an `importance` level that controls confidence decay over time:

| Level | Grace Period | Decay Rate | Floor |
|-------|-------------|------------|-------|
| `core` | ∞ | 0 | 1.0 |
| `important` | 30 days | 0.005/week | 0.3 |
| `normal` | 7 days | 0.01/week | 0.1 |
| `ephemeral` | 1 day | 0.05/week | 0.1 |

### Consolidation

The `Consolidator` finds semantically similar memories (cosine similarity ≥ 0.85),
then uses an LLM to merge them into a single consolidated entry.
Source entries are marked with `merged_into` and excluded from queries but remain
auditable. Triggered via `ozzie memory consolidate`.

### Embedding Pipeline

The `Pipeline` processes embedding jobs asynchronously. When a memory is stored or
updated, an `EmbedJob` is enqueued. The pipeline calls the configured embedding
model, then upserts the vector into the embeddings table. Deletion jobs remove the vector.

## Command Sandbox

The sandbox validates shell commands before execution using **AST-based analysis**
via `mvdan.cc/sh/v3`.

### Architecture

```
User command string
        │
        ▼
  syntax.Parse() → AST
        │
        ▼
  syntax.Walk() traversal
        │
        ├── CallExpr → check binary + flags against denylist
        ├── Stmt     → check redirect targets (>/dev/sd*)
        ├── FuncDecl → detect fork bombs (self-recursive)
        └── CmdSubst/Subshell/ProcSubst → recursive descent
```

### Declarative Denylist

Commands are blocked via a map-based denylist rather than regex patterns:

- **Always blocked**: `mkfs*`, `fdisk`, `sudo`, `doas`, `pkexec`, `su`, `eval`,
  `source`, `.`
- **Blocked with specific flags**: `rm -r/-f`, `chmod -R`, `chown -R`,
  `find -delete/-exec/-execdir`, `dd of=`

### Bypass Coverage

The AST approach covers ~90% of known bypasses vs ~60% with the previous regex:

| Bypass | How it's caught |
|--------|----------------|
| `rm /tmp/foo; rm -rf /` | Each `Stmt` checked independently |
| `rm -r -f /` | All args inspected: flags `{r, f}` |
| `$(rm -rf /)` | Walk descends into `CmdSubst` |
| `(rm -rf /)` | Walk descends into `Subshell` |
| `find / -delete` | Arg values checked alongside flags |
| `eval "rm -rf /"` | `eval` always blocked |
| `$cmd args` | Detected as dynamic command (`ParamExp`) |
| `grep "rm -rf" logfile` | AST knows it's an argument to grep |

## Authentication

Ozzie uses a **local token** scheme for loopback authentication:

1. The gateway generates a 32-byte random token at startup
2. The token is encrypted with the age keyring (`ENC[age:...]`) and written to `$OZZIE_PATH/.local_token`
3. CLI clients (`ozzie ask`, `ozzie tui`) read and decrypt the token, then send it as `Authorization: Bearer <token>`
4. The gateway validates the token using `crypto/subtle.ConstantTimeCompare` (constant-time)
5. The token is cleaned up on gateway shutdown

The `internal/auth/` package provides:
- `Authenticator` interface — extensible for future device pairing (Ed25519)
- `LocalAuth` — local token implementation (generate, encrypt, validate)
- `Middleware` — chi HTTP middleware (nil = insecure passthrough)

Route protection:
- `/api/health` — always public
- `/api/ws`, `/api/events`, `/api/sessions`, `/api/tasks` — behind auth middleware

WebSocket origin check:
- Auth enabled: `OriginPatterns: ["localhost:*", "127.0.0.1:*", "[::1]:*"]`
- `--insecure` flag: `InsecureSkipVerify: true` (dev mode only)

```
$OZZIE_PATH/
├── .local_token     # ENC[age:...] blob, rotated per gateway restart
├── .age/
│   └── current.key  # age private key (clients decrypt token with this)
└── devices/         # (future) paired device public keys
```

## Known Issues (Code Review 2026-03-08)

### Security
- ~~**No WS/HTTP authentication** — Any local process can connect and control Ozzie~~ — **FIXED**: Local token auth via `internal/auth/`
- ~~**Origin check disabled** — `InsecureSkipVerify: true` in WS accept options~~ — **FIXED**: Origin patterns restricted to localhost
- **Template injection** — Env var expansion before JSON parsing can break config structure
- ~~**Session ID entropy** — Truncated to 8 hex chars (~4B combinations)~~ — **FIXED**: Human-readable names via `pkg/names`

### Concurrency
- **Event bus ordering** — `go sub.handler(event)` spawns one goroutine per subscriber per event, breaking delivery order
- **SubscribeChan panic** — Close after unsubscribe races with in-flight goroutines
- **Hub SetTaskHandler** — Written without lock, read concurrently in handleRequest
- **ActorPool preemption** — Force-take after 5s without cancelling the preempted task
- **Goroutine leaks** — Scheduler Stop(), Extractor Stop(), preemption timeout goroutines not awaited

### API
- ~~**`/api/events` no session filter** — Endpoint exposed all events without session filtering~~ — **FIXED**: `?session=...` query param added

### Code Quality
- **`runGateway` god function** — 600+ lines in `cmd/commands/gateway.go`
- **Duplicate code** — consumeRunnerOutput (tasks/skills), preApproveDangerousTools (task/schedule), WS client boilerplate
- **eventrunner.go untested** — 620 lines, 0 tests, most critical file in the agent package

See `local-memo/code-review-2026-03-08.md` for the complete review.

---

## Configuration

### OZZIE_PATH

All Ozzie data lives under a single root directory:
- `$OZZIE_PATH` environment variable, or `~/.ozzie` by default
- Created by `ozzie wake` (onboarding command)

Directory structure:
```
~/.ozzie/
├── config.jsonc      # Main configuration
├── .env              # Environment variables
├── .local_token      # Auth token (ENC[age:...], rotated per restart)
├── .age/             # Age keyring
│   └── current.key   # Active age private key
├── SOUL.md           # Custom persona (optional)
├── logs/             # Application logs
├── sessions/         # Session data (meta.json + messages.jsonl)
├── skills/           # Skill definitions (.jsonc)
├── devices/          # (future) Paired device public keys
└── plugins/          # Plugin manifests and WASM files
```

### JSONC Config

Configuration uses JSONC (JSON with comments) with environment variable templating:

```jsonc
{
  "gateway": { "host": "127.0.0.1", "port": 4242 },
  "models": {
    "default": "main",
    "providers": {
      "main": {
        "driver": "anthropic",
        "model": "claude-sonnet-4-20250514",
        "auth": { "api_key": "${ANTHROPIC_API_KEY}" }
      }
    }
  }
}
```

The `${{ .Env.VAR }}` syntax is resolved at load time via Go template expansion.
Defaults are applied for missing fields (port 4242, buffer size 1000, etc.).
