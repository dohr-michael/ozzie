# Packaging Rules

## Semantic grouping

Every package belongs under its business domain:

- `internal/agent/` — agent runtime, factory, callbacks, middlewares
- `internal/events/` — event bus + event logger
- `internal/sessions/` — session store + cost tracker
- `internal/connectors/` — connector manager + connector implementations (e.g. `discord/`)
- `internal/memory/` — memory wiring sub-packages (`bridge/`, `layered/`)
- `internal/plugins/` — tool registry, native tools, WASM, MCP, dangerous wrapper, sandbox
- `pkg/memory/` — importable memory library (SQLite, vector, tools)
- `pkg/connector/` — importable connector interfaces
- `pkg/names/` — importable ID generation

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
