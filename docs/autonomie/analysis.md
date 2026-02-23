# Autonomy Gap Analysis

> Date: 2026-02-23
> Baseline: Phase 1a complete + Foundations F1–F7 done

## Current State

Ozzie has a solid foundation: event bus, session persistence, plugin system
(WASM + native), declarative workflow skills (DAG), dynamic prompt composition,
context compression, cost tracking, and event persistence.

The E2E flow works: `ozzie gateway` → `ozzie ask "hello"` → streamed LLM
response with tool use, session history, and context management.

## What's Missing for Autonomy

An autonomous agent must be able to: receive a complex task, decompose it,
execute sub-tasks (potentially in parallel), survive crashes, report progress,
and learn from experience — all without requiring the user to stay connected.

### Tier 1 — Critical (Phase 2a)

These are blocking. Without them, Ozzie cannot handle any task that exceeds a
single request-response cycle.

| Gap | Problem | Impact |
|-----|---------|--------|
| **Persistent Task Queue** | No concept of a "task" beyond a single message turn. No persistence, no progress, no checkpointing. | Cannot handle long-running work. User must stay connected. |
| **Heartbeat & Supervision** | No liveness detection. Gateway crash = silent death. No auto-restart, no recovery of in-flight tasks. | Unreliable for any unattended operation. |
| **Async Sub-agents** | Skills/workflows execute synchronously, blocking the main agent. No notification on completion. | Cannot work on multiple things. Cannot delegate and continue. |

### Tier 2 — Important (Phase 2b)

These unlock intelligence and proactive behavior.

| Gap | Problem | Impact |
|-----|---------|--------|
| **Scheduler / Triggers** | `internal/scheduler/` is a stub. `schedule.trigger` event type defined but never produced. | Cannot schedule recurring tasks or react to external events. |
| **Long-term Memory** | No structured memory across sessions. Each conversation starts from zero (except session history). | Cannot learn from past mistakes. Cannot remember user preferences durably. |
| **Plan-Execute-Verify Loop** | Workflow skills have `acceptance` criteria but no automated verification or retry loop. | Cannot self-correct. Cannot handle tasks requiring iteration. |

### Tier 3 — Useful (Phase 2c)

These improve safety and code-specific autonomy.

| Gap | Problem | Impact |
|-----|---------|--------|
| **Sandbox / Worktrees** | `cmd`, `run_command`, `root_cmd` execute on host. No isolation for autonomous code execution. | Risk of unintended damage when agent operates unsupervised. |
| **Project Indexing** | No automatic understanding of codebase structure, technologies, conventions. | Agent must re-discover project context every session. |
| **Post-task Reflection** | No self-evaluation mechanism after task completion. | Cannot accumulate operational wisdom. |

## Architecture Observation

The current architecture is **reactive**: `EventRunner` subscribes to
`user.message` events and produces responses. The agent only acts when spoken
to.

An autonomous agent needs a **proactive execution layer** on top of the
reactive one:

```
┌──────────────────────────────────────────────┐
│  Proactive Layer (Phase 2a)                  │
│  ┌─────────┐  ┌──────────┐  ┌────────────┐  │
│  │  Task   │  │ Worker   │  │ Heartbeat  │  │
│  │  Queue  │──│ Pool     │──│ Monitor    │  │
│  └─────────┘  └──────────┘  └────────────┘  │
├──────────────────────────────────────────────┤
│  Reactive Layer (Phase 1, exists)            │
│  ┌─────────┐  ┌──────────┐  ┌────────────┐  │
│  │  Event  │  │  Event   │  │  Agent     │  │
│  │  Bus    │──│  Runner  │──│  (Eino)    │  │
│  └─────────┘  └──────────┘  └────────────┘  │
└──────────────────────────────────────────────┘
```

The proactive layer submits work to the reactive layer via the event bus. The
reactive layer doesn't need to change — it just gets a new source of events
(tasks) alongside user messages.
