# Packaging Rules

## Semantic grouping

Every package belongs under its business domain:

- `internal/brain/` — pure domain: ports (`Tool`, `Runner`, `Message`…), `ToolSet`, `ModelTier`
- `internal/brain/events/` — event bus + event logger
- `internal/brain/prompt/` — prompt template registry, composer, section builders
- `internal/brain/actors/` — capacity-aware actor pool
- `internal/brain/memory/layered/` — L0/L1/L2 context compression
- `internal/brain/skills/` — DAG workflow engine
- `internal/brain/conscience/` — dangerous wrapper, sandbox, permissions, constraints
- `internal/brain/introspection/` — logger setup, log level resolution
- `internal/agent/` — Eino ADK adapter (agent runtime, factory, callbacks, middlewares)
- `internal/membridge/` — memory wiring adapter (embedder factory, cross-task extractor)
- `internal/eyes/` — connector manager + connector implementations (e.g. `discord/`)
- `internal/hands/` — tool registry, native tools, WASM, MCP wrappers
- `internal/sessions/` — session store + cost tracker
- `pkg/memory/` — importable memory library (SQLite, vector, tools)
- `pkg/connector/` — importable connector interfaces
- `pkg/names/` — importable ID generation

## DDD Hexagonal rule

`brain/` is a **pure domain** package — no imports of Eino (`cloudwego/eino`), `internal/agent`, `internal/hands`, or `internal/models`. Domain interfaces (ports) live in `brain/ports.go`. Infrastructure adapters implement those ports in `internal/agent/` and `internal/membridge/`.

## `pkg/` vs `internal/`

- **`pkg/`** = reusable libraries with **no dependency** on Ozzie internal wiring (config, events, sessions…)
- **`internal/`** = everything else (application logic, wiring, infra)

## No orphan packages

A package with 1-2 files and a single consumer should be merged into its consumer.

## `clients/`

Only user-facing clients live here: `clients/ws/`, `clients/tui/`. Internal interface implementations (e.g. Discord connector) belong in `internal/`.

## Sub-packages

Use sub-packages for large families (`memory/`, `connectors/`) but avoid fragmentation — no sub-package for < 3 files.

## `internal/storage/`

Only `dirstore/` lives here (generic directory-based persistence). Domain-specific persistence belongs in its domain package.
