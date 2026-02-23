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

## Current state (Phase 1a — done)

Working E2E flow: `ozzie gateway` → `ozzie ask "hello"` → streamed LLM response.

Stub packages exist for: plugins, skills, sessions, scheduler, storage, TUI.

## Key files

| What            | Where                                               |
|-----------------|-----------------------------------------------------|
| Entry point     | `cmd/ozzie/main.go`                                 |
| CLI commands    | `cmd/commands/`                                     |
| Config          | `internal/config/` + `configs/config.example.jsonc` |
| Event bus       | `internal/events/`                                  |
| Models          | `internal/models/` (registry, anthropic, openai)    |
| Gateway         | `internal/gateway/` (chi server + WS hub)           |
| Agent           | `internal/agent/` (Eino ADK + EventRunner)          |
| WS client       | `clients/ws/`                                       |
| Default persona | `internal/agent/agent.go` → `DefaultPersona` + `AgentInstructions` |
