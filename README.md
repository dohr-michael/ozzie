# Ozzie

**Your personal AI agent operating system.**

> *Connect everything. Trust nothing.*

Ozzie is a self-hosted, event-driven agent gateway that connects LLMs to any tool, messaging platform, or workflow —
with zero-trust security baked in from day one.

Named after [Ozzie Isaacs](https://en.wikipedia.org/wiki/Commonwealth_Saga) from Peter F. Hamilton's *Commonwealth
Saga* — co-inventor of wormholes, creator of Sentient Intelligence, and architect of the Gaiafield.

> *" Ozzie would have loved to dive right in and give himself a decent clean, but even though they still hadn't seen a
single living creature on this world, he just couldn't quite bring himself to trust the water. Too many late student
nights with a pizza, a couple of six-packs, some grass, and a bad sci-fi DVD. God only knew what lurked along the bottom
of the river, maybe nothing, but he certainly wasn't going to wind up with alien eggs hatching out of his ass, thank
you. "*
> — Peter F. Hamilton, *Pandora's Star*

---

## Why Ozzie?

Most AI agent frameworks let plugins access your filesystem, leak secrets at runtime, and mix instructions with
executable code. Ozzie takes a different approach:

- **Zero-trust by default** — Wasm-sandboxed plugins with no host access unless explicitly granted via capabilities
- **Gateway pattern** — One persistent process (`ozzie gateway`) orchestrates everything; clients (TUI, CLI, web)
  connect ephemerally via WebSocket
- **Event-driven backbone** — Every action (message, decision, tool call) is an immutable event — auditable, replayable,
  human-readable
- **Multi-LLM** — 4 drivers (Anthropic, OpenAI, Mistral, Ollama) with lazy-init registry, provider cooldown, and
  automatic task retry on failure
- **Agent autonomy** — Coordinator pattern (explore → plan → validate → execute), async task delegation, ephemeral
  sub-agents, cron/event scheduler
- **Semantic memory** — Hybrid retrieval (keyword + vector embeddings), cross-task learning, automatic memory extraction
- **3-level plugin architecture** — Communication (senses), Tools (hands), Skills (expertise) — each with distinct
  isolation boundaries
- **MCP server** — Expose all Ozzie tools as an MCP server for Claude Code or any MCP-compatible client
- **Container-ready** — Multi-arch Docker images, goreleaser builds, GitHub Actions CI/CD

## Architecture

```
┌───────────────────────────────────────────────┐
│           GATEWAY  (ozzie gateway)            │
│           127.0.0.1:18420                     │
├───────────────────────────────────────────────┤
│  Agent Core (Eino ADK)                        │
│  ├─ Coordinator (explore/plan/validate/exec)  │
│  ├─ Actor Pool (capacity-aware, cooldown)     │
│  ├─ Task Runner (async, suspend/resume)       │
│  └─ Memory (hybrid keyword + vector)          │
│                                               │
│  Event Bus (36 typed events, ring buffer)     │
│  Plugin Router (Extism WASM + native tools)   │
│  Skill Engine (DAG workflows, acceptance)     │
│  Scheduler (cron, event triggers, cooldown)   │
│  MCP Server (stdio)                           │
└────────────────────┬──────────────────────────┘
                     │ WebSocket
           ┌─────────┼─────────┐
           ↓         ↓         ↓
         TUI       CLI       Web
      (planned)  (one-shot)  (planned)
```

## Quick Start

```bash
# Configure
cp configs/config.example.jsonc configs/config.jsonc
export ANTHROPIC_API_KEY="sk-..."

# Build & run
make build
./build/ozzie gateway

# In another terminal
./build/ozzie ask "Hello, who are you?"
```

### Docker

```bash
# Build
go build -o build/ozzie ./cmd/ozzie
docker build -t ozzie .

# Run (mount Docker socket for container-based tasks)
docker run -d \
  -v ~/.ozzie:/home/ozzie/.ozzie \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -p 18420:18420 \
  ozzie
```

## Agent System

### Coordinator Pattern

Ozzie uses a 5-phase coordinator for complex tasks:

1. **Explore** — Gather context, read files, understand the problem
2. **Plan** — Design an approach based on findings
3. **Validate** — Request human approval (supervised mode) or auto-approve (autonomous mode)
4. **Execute** — Run the plan via sub-agents with tool access
5. **Verify** — Check results against acceptance criteria

Modes: `disabled` (direct execution), `supervised` (human-in-the-loop), `autonomous` (full autopilot).

### Async Tasks

```bash
# Submit a background task
./build/ozzie ask "Refactor the auth module"

# Check task status
./build/ozzie tasks list
./build/ozzie tasks get <task-id>
```

Tasks run in isolated sub-agents with their own tool sets, crash recovery, and heartbeat monitoring.

### Memory

Ozzie maintains a persistent semantic memory system:

- **Hybrid retrieval** — Keyword scoring (tag match, title match, recency) blended with vector cosine similarity
- **Async embedding pipeline** — Memory storage returns immediately; vector indexing happens in background
- **Cross-task learning** — Sub-agents automatically receive relevant memories at startup (deterministic injection)
- **Auto-extraction** — Completed tasks are analyzed to extract reusable lessons (preferences, facts, procedures)

### Skills

Declarative workflows defined in JSONC:

- **Simple skills** — Single-step tool execution with parameters
- **Workflow skills** — Multi-step DAG with dependencies and parallel execution
- **Triggers** — Cron schedules, event-based activation, manual invocation
- **Acceptance criteria** — LLM-verified output validation

## Plugins

Ozzie plugins are WASM modules with a JSON manifest. Each plugin can expose **one or more tools**.

### Plugin manifest

```jsonc
{
    "name": "my_plugin",
    "description": "What the plugin does",
    "provider": "extism",
    "wasm_path": "my_plugin.wasm",
    "dangerous": false,
    "capabilities": {
        // Deny-by-default — grant only what's needed
        "http": { "allowed_hosts": ["api.example.com"] },
        "kv": true,
        "log": true
    },
    "tools": [
        {
            "name": "list_items",
            "description": "List all items",
            "func": "list_items",
            "parameters": {}
        },
        {
            "name": "create_item",
            "description": "Create a new item",
            "func": "create_item",
            "dangerous": true,
            "parameters": {
                "name": { "type": "string", "description": "Item name", "required": true }
            }
        }
    ]
}
```

Each tool maps to a WASM export function. Single-tool plugins can omit `func` (defaults to `"handle"`).

### Building plugins

Plugins are built with [TinyGo](https://tinygo.org/) targeting `wasip1`:

```bash
make build-plugins
```

### Plugin catalog

**WASM plugins:**

| Plugin | Tools | Description |
|--------|-------|-------------|
| `calculator` | `calculator` | Evaluate math expressions |
| `todo` | `todo` | Task list (add/list/done/remove) with KV storage |
| `web_crawler` | `web_crawler` | Fetch and extract text from web pages |
| `patch` | `patch` | Apply unified diff patches to files |

**Native tools (built-in):**

| Tool | Description |
|------|-------------|
| `run_command` | Execute shell commands |
| `submit_task` | Delegate work to async sub-agents |
| `check_task` / `cancel_task` / `list_tasks` | Task lifecycle management |
| `plan_task` / `reply_task` | Coordinator pattern tools |
| `request_validation` | Request human approval |
| `store_memory` / `query_memories` / `forget_memory` | Persistent semantic memory |
| `update_session` | Update session metadata |
| `schedule_task` / `unschedule_task` / `list_schedules` | Dynamic scheduling |
| `activate_tools` | Dynamically enable WASM plugin tools |

**Filesystem tools (via Eino ADK middleware):**

`ls`, `read_file`, `write_file`, `edit_file`, `glob`, `grep`

## MCP Server

Ozzie can expose its tools as an [MCP](https://modelcontextprotocol.io/) server over stdio, making them available to Claude Code or any MCP-compatible client.

```bash
# Expose all tools
ozzie mcp-serve

# Expose a specific plugin or tool
ozzie mcp-serve calculator
```

### Claude Code integration

Add to your `.mcp.json` (project root or `~/.claude/.mcp.json`):

```json
{
    "mcpServers": {
        "ozzie": {
            "type": "stdio",
            "command": "./build/ozzie",
            "args": ["mcp-serve", "--config", "./configs/config.dev.jsonc"],
            "env": {
                "OZZIE_PATH": "./build/ozzie-home"
            }
        }
    }
}
```

The `OZZIE_PATH` env var isolates dev data from `~/.ozzie`. To expose a single plugin, add its name:
`"args": ["mcp-serve", "--config", "./configs/config.dev.jsonc", "calculator"]`

Tools marked `dangerous` are **not** wrapped with confirmation in MCP mode — the MCP client handles its own safety.

## Development

```bash
make build          # Build the binary
make build-plugins  # Build WASM plugins (requires TinyGo)
make test           # Run tests
make staticcheck    # Run staticcheck linter
make check          # All quality gates (build + lint + test)
make clean          # Clean build artifacts
make run-gateway    # Build and run the gateway
make run-ask        # Build and run ask with a test message
```

### Quality gates

Every change must pass all three — no exceptions:

```bash
go build ./...              # compile
~/go/bin/staticcheck ./...  # lint
go test ./...               # tests
```

### Plugin development with Claude Code

Use the `/dev-plugin` slash command to start an interactive plugin development session with MCP:

```
/dev-plugin my_plugin "Description of what the plugin does"
```

This sets up the MCP server, scaffolds the plugin structure, and iterates on the implementation with live testing.

## Tech Stack

| Component     | Choice               | Why                                                 |
|---------------|----------------------|-----------------------------------------------------|
| Language      | **Go 1.25**          | Single binary, goroutines, fast compilation         |
| Router        | **go-chi**           | Lightweight, idiomatic, middleware support           |
| LLM Framework | **Eino** (CloudWeGo) | Type-safe, streaming-first, Deep Agent, MCP native  |
| Sandbox       | **Extism** (wazero)  | Pure Go, no CGo, capabilities-based isolation       |
| MCP SDK       | **go-sdk**           | Official MCP Go SDK, stdio transport                |
| Event Bus     | In-memory channels   | Typed payloads, ring buffer, session-scoped routing |
| Config        | **JSONC**            | Comments in JSON, env variable templating           |
| Build/Release | **goreleaser**       | Multi-arch binaries + Docker images + manifests     |
| CI/CD         | **GitHub Actions**   | Quality gates + snapshot + semver release           |

## Roadmap

### Done

- [x] **Phase 1 — Foundations** — Gateway, WS hub, Event bus (36 types), CLI, Config (JSONC + env templating)
- [x] **Phase 1a — Agent Core** — Eino ADK agent, coordinator pattern, 13+ native tools, async tasks, actor pool
- [x] **Phase 1b — Ecosystem** — Skills (DAG workflows), scheduler (cron/event), WASM plugins (Extism), MCP server
- [x] **Phase 2a — Intelligence** — Semantic memory (hybrid keyword + vector), cross-task learning, auto-extraction
- [x] **Phase 3a — Containerization** — Dockerfile, goreleaser multi-arch, GitHub Actions CI/CD, runtime awareness

### In Progress

- [ ] **SLM optimization** — Prompt tuning for smaller local models (Ollama), instruction compression
- [ ] **WASM hardening** — CPU/memory limits (Extism fuel metering), timeout enforcement, hostile input testing

### Planned

- [ ] **Sub-agent persistence** — State continuity across tasks (beyond current checkpoint system)
- [ ] **TUI client** — Interactive terminal UI (Bubbletea)
- [ ] **Web portal** — SPA for session management and monitoring
- [ ] **Integrations** — Discord, Telegram, WhatsApp connectors
- [ ] **Distributed events** — NATS-backed event bus for multi-instance deployments

### Long-term Vision

Ozzie aims to be a **personal AI operating system** — a single self-hosted process that:

1. **Orchestrates any LLM** — Cloud (Anthropic, OpenAI, Mistral) and local (Ollama) models with automatic failover,
   provider cooldown, and capacity-aware routing
2. **Learns continuously** — Semantic memory accumulates across conversations and tasks; sub-agents inherit relevant
   context automatically
3. **Operates autonomously** — The coordinator pattern enables supervised-to-autonomous progression; cron/event
   scheduling drives proactive behavior
4. **Stays secure** — Zero-trust WASM sandbox for plugins, deny-by-default capabilities, destructive command blocking
   in autonomous mode
5. **Connects everywhere** — MCP server for IDE integration, WebSocket for custom clients, messaging connectors for
   human-facing channels

The end state: an always-on agent that manages your dev environment, automates workflows, and grows smarter with every
interaction — while you keep full control over what it can access and do.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
