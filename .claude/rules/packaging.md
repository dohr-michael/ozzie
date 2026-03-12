# Packaging Rules

## Three-layer layout

```
internal/config/   → shared config (neither core nor infra)
internal/core/     → pure domain (zero infra dependency)
internal/infra/    → adapters & infrastructure
pkg/               → importable libraries (no internal/ dependency)
cmd/               → assembly & CLI commands
clients/           → user-facing clients
```

## Semantic grouping

### `internal/core/` — Pure domain

- `core/brain/` — domain ports (`Tool`, `Runner`, `Message`…), `ToolSet`, `ModelTier`
- `core/events/` — event bus + event logger
- `core/prompt/` — prompt template registry, composer, section builders
- `core/actors/` — capacity-aware actor pool
- `core/layered/` — L0/L1/L2 context compression
- `core/skills/` — DAG workflow engine
- `core/conscience/` — dangerous wrapper, sandbox, permissions, constraints
- `core/introspection/` — logger setup, log level resolution
- `core/policy/` — pairing policy

### `internal/infra/` — Adapters & infrastructure

- `infra/agent/` — Eino ADK adapter (agent runtime, factory, callbacks, middlewares)
- `infra/models/` — LLM model registry (Anthropic, OpenAI, Gemini, Mistral, Ollama)
- `infra/membridge/` — memory wiring adapter (embedder factory, cross-task extractor)
- `infra/hands/` — tool registry, native tools, WASM, MCP wrappers
- `infra/gateway/` — HTTP server (chi) + WebSocket hub
- `infra/sessions/` — session store + cost tracker
- `infra/tasks/` — async task runner + file store
- `infra/scheduler/` — cron + event-based scheduler
- `infra/eyes/` — connector manager + connector implementations (e.g. `discord/`)
- `infra/mcp/` — MCP server
- `infra/storage/dirstore/` — generic directory-based persistence
- `infra/secrets/` — secret management
- `infra/auth/` — authentication middleware
- `infra/heartbeat/` — heartbeat monitoring
- `infra/i18n/` — internationalization
- `infra/ui/` — UI components + setup wizard

### `pkg/` — Importable libraries

- `pkg/memory/` — importable memory library (SQLite, vector, tools)
- `pkg/llmutil/` — LLM response utilities
- `pkg/connector/` — importable connector interfaces
- `pkg/names/` — importable ID generation
- `pkg/editor/` — editor integration
- `pkg/htmltext/` — HTML text utilities

## DDD Hexagonal rule

`core/` is a **pure domain** layer — no imports of Eino (`cloudwego/eino`), `infra/`, or any infrastructure package. Domain interfaces (ports) live in `core/brain/ports.go`. Infrastructure adapters implement those ports in `infra/agent/` and `infra/membridge/`.

**Dependency rule**: `core/` → never imports `infra/`. `infra/` → may import `core/`. `pkg/` → never imports `internal/`.

## `internal/config/`

Stays at `internal/config/` (top-level, neither `core/` nor `infra/`). Imported by both layers. Contains DTOs — neither purely domain nor purely infrastructure.

## `pkg/` vs `internal/`

- **`pkg/`** = reusable libraries with **no dependency** on Ozzie internal wiring (config, events, sessions…)
- **`internal/`** = everything else (application logic, wiring, infra)

## No orphan packages

A package with 1-2 files and a single consumer should be merged into its consumer.

## `clients/`

Only user-facing clients live here: `clients/ws/`, `clients/tui/`. Internal interface implementations (e.g. Discord connector) belong in `internal/`.

## Sub-packages

Use sub-packages for large families (`eyes/`, `gateway/`) but avoid fragmentation — no sub-package for < 3 files.
