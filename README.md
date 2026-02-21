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
> — Peter F. Hamilton, *Pandora'Star*

---

## Why Ozzie?

Most AI agent frameworks let plugins access your filesystem, leak secrets at runtime, and mix instructions with
executable code. Ozzie takes a different approach:

- **Zero-trust by default** — Wasm-sandboxed plugins with no host access unless explicitly granted via capabilities
- **Gateway pattern** — One persistent process (`ozzie gateway`) orchestrates everything; clients (TUI, CLI, web)
  connect ephemerally via WebSocket
- **Event-driven backbone** — Every action (message, decision, tool call) is an immutable event — auditable, replayable,
  human-readable
- **3-level plugin architecture** — Communication (senses), Tools (hands), Skills (expertise) — each with distinct
  isolation boundaries
- **Multi-tool plugins** — A single WASM plugin can expose multiple tools (e.g. a "controlm" plugin with 13 tools)
- **MCP server** — Expose all Ozzie tools as an MCP server for Claude Code or any MCP-compatible client
- **Tag-based model routing** — Select models by security tier, cost, speed, or jurisdiction — not by hard-coded
  provider names
- **Language-agnostic** — Write plugins in Rust, Go, JS, or any language that compiles to Wasm

## Architecture

```
┌─────────────────────────────────────────┐
│         GATEWAY  (ozzie gateway)        │
│         127.0.0.1:18420                 │
├─────────────────────────────────────────┤
│  Agent Core (Eino)                      │
│  Event Bus + Store                      │
│  Plugin Router (Extism + MCP)           │
│  Scheduler (heartbeat, cron, triggers)  │
└──────────────────┬──────────────────────┘
                   │ WebSocket
         ┌─────────┼─────────┐
         ↓         ↓         ↓
       TUI       CLI       Web
    (Bubbletea) (one-shot)  (SPA)
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

### Existing plugins

| Plugin | Tools | Description |
|--------|-------|-------------|
| `calculator` | `calculator` | Evaluate math expressions |
| `todo` | `todo` | Task list (add/list/done/remove) with KV storage |
| `web_crawler` | `web_crawler` | Fetch and extract text from web pages |
| `patch` | `patch` | Apply unified diff patches to files |
| `cmd` (native) | `cmd` | Execute shell commands |
| `root_cmd` (native) | `root_cmd` | Execute commands with sudo |

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

### Plugin development with Claude Code

Use the `/dev-plugin` slash command to start an interactive plugin development session with MCP:

```
/dev-plugin my_plugin "Description of what the plugin does"
```

This sets up the MCP server, scaffolds the plugin structure, and iterates on the implementation with live testing.

## Tech Stack

| Component     | Choice               | Why                                                 |
|---------------|----------------------|-----------------------------------------------------|
| Language      | **Go**               | Single binary, goroutines, fast compilation         |
| Router        | **go-chi**           | Lightweight, idiomatic, middleware support          |
| LLM Framework | **Eino** (CloudWeGo) | Type-safe, streaming-first, Deep Agent, MCP native  |
| Sandbox       | **Extism** (wazero)  | Pure Go, no CGo, capabilities-based isolation       |
| MCP SDK       | **go-sdk**           | Official MCP Go SDK, stdio transport                |
| Event Bus     | In-memory channels   | Proven pattern, typed payloads, ring buffer history |
| Config        | **JSONC**            | Comments in JSON, env variable templating           |

## Roadmap

- [x] **Phase 1a** — Gateway + Event Bus + Model Registry + CLI `ask` (E2E flow)
- [x] **Phase 1b** — Extism runtime + plugin system + tool calling
- [x] **Phase 1b+** — Multi-tool plugins + MCP server
- [ ] **Phase 1c** — TUI client
- [ ] **Phase 1d** — Skill loader + Telegram connector + Scheduling
- [ ] **Phase 2** — Web portal + Vector search + Multi-agent workflows

## License

TBD
