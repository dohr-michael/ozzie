# Autonomy Gap Analysis

> Date: 2026-02-23, **Updated 2026-03-08**
> Baseline: Phase 1a complete + Foundations F1–F7 done

## Status: All 3 tiers implemented

All autonomy gaps from the original analysis have been addressed:

### Tier 1 — Critical ✅ DONE

| Gap | Resolution |
|-----|-----------|
| **Persistent Task Queue** | `internal/tasks/` — Full lifecycle (create-execute-verify-suspend/resume), FileStore persistence |
| **Heartbeat & Supervision** | `internal/heartbeat/` — Liveness detection + `internal/actors/pool.go` with recovery middleware |
| **Async Sub-agents** | Actor pool with capacity management, ephemeral TaskRunner per task, pre-approval for dangerous tools |

### Tier 2 — Important ✅ DONE

| Gap | Resolution |
|-----|-----------|
| **Scheduler / Triggers** | `internal/scheduler/` — Cron + interval + event triggers, cooldown, max_runs (83.8% test coverage) |
| **Long-term Memory** | `internal/memory/` — Hybrid keyword+vector, async pipeline, extraction, decay, dedup (78.4% coverage) |
| **Plan-Execute-Verify Loop** | Skills DAG with acceptance criteria + verifier, coordinator pattern |

### Tier 3 — Useful ✅ DONE

| Gap | Resolution |
|-----|-----------|
| **Sandbox / Worktrees** | WASM sandbox, path jail, destructive pattern blocking, write-path sandbox |
| **Project Indexing** | Dynamic prompt composition with tool/skill manifests |
| **Post-task Reflection** | Memory extractor auto-extracts lessons from task.completed events |

## Remaining Quality Issues (from code review)

While all features are implemented, the code review (2026-03-08) found:

- **Actor pool race conditions** — preemption force-take without cancel, goroutine leaks
- **Scheduler goroutine leaks** — Stop() doesn't await cronLoop/intervalLoop
- **Memory extractor leaks** — Goroutines launched without WaitGroup
- **I/O under mutex** — Actor pool schedule() reads disk under lock

See `local-memo/code-review-2026-03-08.md` for details.
