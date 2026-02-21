---
description: Ozzie architecture rules and conventions
globs: "**/*.go"
---

# Architecture

Ozzie follows the **gateway pattern**: one persistent process (`ozzie gateway`) orchestrates everything, clients connect via WebSocket.

## Package layout

```
cmd/ozzie/          → entry point
cmd/commands/       → CLI commands (urfave/cli v3)
internal/config/    → JSONC config loader
internal/events/    → event bus (in-memory channels, typed payloads)
internal/models/    → LLM model registry (Anthropic, OpenAI, Ollama via Eino)
internal/gateway/   → HTTP server (chi) + WebSocket hub
internal/agent/     → Eino ADK agent + EventRunner
internal/callbacks/ → Eino callbacks → event bus bridge
clients/ws/         → WS client library
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
- **Models**: lazy-initialized via Registry. Auth resolved from config → env var → driver default. Ollama needs no auth (local)
- **CLI v3**: `ActionFunc` is `func(context.Context, *Command) error` — no `*cli.Context`
- **WebSocket**: uses `github.com/coder/websocket` (NOT nhooyr.io which is deprecated)
- **Eino ADK**: `ChatModelAgentConfig` requires `Name`, `Description`, uses `Instruction` (not SystemPrompt). `NewRunner` takes `RunnerConfig` struct. `Runner.Run` takes `[]adk.Message` (alias for `[]*schema.Message`)
