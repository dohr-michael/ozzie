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
| `internal/gateway/` | HTTP server (chi), routes | `events`, `gateway/ws`, `sessions` |
| `internal/gateway/ws/` | WebSocket hub, client management, protocol frames | `events`, `sessions` |
| `internal/agent/` | Eino ADK agent, EventRunner, prompt composition | `events`, `sessions`, `config` |
| `internal/callbacks/` | Eino callbacks → event bus bridge | `events` |
| `internal/plugins/` | Plugin system (WASM + native), manifest, tool registry | — |
| `internal/skills/` | Skill system (simple + workflow DAG), skill-as-tool | `agent`, `events`, `models`, `plugins` |
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
- **History**: ring buffer stores recent events for the `/api/events` endpoint
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

Skills are declarative agent behaviors defined in JSONC files.

### Simple Skills

A simple skill wraps a single LLM agent with a specific instruction:

```jsonc
{
  "name": "code-review",
  "description": "Review code for quality issues",
  "type": "simple",
  "instruction": "Review the provided code...",
  "model": "claude-sonnet",
  "tools": ["read_file", "search"]
}
```

### Workflow Skills

Workflow skills define a DAG (directed acyclic graph) of steps:

```jsonc
{
  "name": "deploy",
  "description": "Deploy pipeline",
  "type": "workflow",
  "vars": {
    "env": { "description": "Target environment", "required": true }
  },
  "steps": [
    { "id": "build", "instruction": "Build the project." },
    { "id": "test",  "instruction": "Run tests.", "needs": ["build"] },
    { "id": "deploy","instruction": "Deploy to {{env}}.", "needs": ["test"] }
  ]
}
```

Steps execute in parallel where the dependency graph allows. The `WorkflowRunner`
uses Kahn's algorithm for topological ordering and `ReadySteps()` to determine
which steps can run concurrently.

### Skill-as-Tool Pattern

Skills are exposed to the main agent as tools via `SkillTool`. When the agent
decides to invoke a skill, it calls the tool, which:

1. Emits `skill.started` event
2. Creates an ephemeral agent (simple) or runs the DAG (workflow)
3. Emits `skill.completed` event with output and duration

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
├── SOUL.md           # Custom persona (optional)
├── logs/             # Application logs
├── sessions/         # Session data (meta.json + messages.jsonl)
├── skills/           # Skill definitions (.jsonc)
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
