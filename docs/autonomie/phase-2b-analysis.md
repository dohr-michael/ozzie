# Phase 2b — Intelligence & Proactive Behavior

> Date: 2026-02-23
> Baseline: Phase 2a complete (F8 Task Queue, F9 Heartbeat, F10 Async Sub-agents)
> Tier: 2 — Important

## Summary

Phase 2a gave Ozzie the ability to **execute** autonomously: persistent tasks,
crash recovery, background workers. Phase 2b gives Ozzie the ability to **think**
autonomously: act without being asked, learn from experience, and self-correct.

Three foundations:

| ID | Foundation | One-liner |
|----|-----------|-----------|
| **F11** | Scheduler & Triggers | Ozzie acts without being asked |
| **F12** | Long-term Memory | Ozzie remembers across sessions |
| **F13** | Plan-Execute-Verify | Ozzie self-corrects |

---

## F11: Scheduler & Event-Triggered Execution

### The Problem

Ozzie is purely reactive. `EventRunner` subscribes to `user.message` and
`task.completed` — nothing else triggers agent work. The scheduler package
(`internal/scheduler/`) is an empty stub. The `schedule.trigger` event type is
defined in `bus.go` but never produced.

**Consequence:** No recurring tasks, no time-based autonomy, no event chains.
The user must manually trigger every action.

### What Exists

| Component | State | Location |
|-----------|-------|----------|
| `schedule.trigger` event type | Defined, never emitted | `internal/events/bus.go:49` |
| Scheduler package | Empty stub (`doc.go` only) | `internal/scheduler/` |
| Skill `TriggerConfig` | Only `Delegation bool` | `internal/skills/skill.go:36` |
| Task worker pool | Polls `TaskPending`, dependencies resolved | `internal/tasks/worker.go` |
| Cron library | Not in `go.mod` | — |

### What Needs to Change

#### 1. Extend `TriggerConfig` in skills

Current:
```go
type TriggerConfig struct {
    Delegation bool `json:"delegation"`
}
```

Target:
```go
type TriggerConfig struct {
    Delegation bool              `json:"delegation"`
    Cron       string            `json:"cron,omitempty"`       // "0 9 * * *"
    OnEvent    *EventTrigger     `json:"on_event,omitempty"`
    Keywords   []string          `json:"keywords,omitempty"`   // future: keyword matching
}

type EventTrigger struct {
    EventType string `json:"event"`                            // "task.completed"
    Filter    map[string]string `json:"filter,omitempty"`      // payload key=value match
}
```

Skills with `cron` or `on_event` are automatically registered with the scheduler.
No change to how skills are loaded or validated — just new optional fields.

**Files:** `internal/skills/skill.go` (edit)

#### 2. Implement the Scheduler

The scheduler is a long-lived goroutine started in `gateway.go`. It has two
responsibilities: time-based triggers (cron) and event-based triggers.

```
┌──────────────────────────────┐
│         Scheduler            │
│                              │
│  ┌────────────┐  ┌────────┐ │
│  │ CronRunner │  │ Event  │ │
│  │ (ticker)   │  │ Matcher│ │
│  └──────┬─────┘  └───┬────┘ │
│         │            │      │
│         └─────┬──────┘      │
│               ▼             │
│        submit_task()        │
│        or run skill         │
└──────────────────────────────┘
```

**CronRunner:** Wakes every minute. Iterates registered cron entries. If an
entry matches the current time, submits a task to the worker pool (reuses
`tasks.WorkerPool.Submit`).

**EventMatcher:** Subscribes to the event bus. For each event, checks registered
`on_event` triggers. If a trigger matches (event type + optional payload filter),
submits the corresponding skill as a task.

```go
type Scheduler struct {
    pool      *tasks.WorkerPool
    bus       *events.Bus
    skills    *skills.Registry
    entries   []Entry
    ctx       context.Context
    cancel    context.CancelFunc
}

type Entry struct {
    SkillName string
    Cron      *CronExpr       // parsed cron expression
    OnEvent   *EventTrigger
    LastRun   time.Time
    Vars      map[string]string // default vars for the skill
}
```

**Cron parsing:** Use a minimal cron expression parser (5-field: min hour dom mon
dow). Either vendored or a lightweight lib like `github.com/robfig/cron/v3`
(parser only, not the scheduler — we manage our own goroutine).

**Files:**
- `internal/scheduler/scheduler.go` — main Scheduler, Start/Stop, cron loop
- `internal/scheduler/cron.go` — cron expression parser + matcher
- `internal/scheduler/matcher.go` — event trigger matching
- `internal/scheduler/scheduler_test.go`
- `internal/scheduler/cron_test.go`

#### 3. Wire into gateway

After worker pool start, before server start:

```go
sched := scheduler.NewScheduler(scheduler.Config{
    Pool:     pool,
    Bus:      bus,
    Skills:   skillRegistry,
})
sched.Start()
defer sched.Stop()
```

**Files:** `cmd/commands/gateway.go` (edit)

#### 4. CLI commands

```
ozzie schedule list           # show registered triggers + next run
ozzie schedule history        # recent trigger executions
```

Read-only. Scheduling is declared in skill JSONC, not via CLI.

**Files:** `cmd/commands/schedule.go` (create), `cmd/commands/root.go` (edit)

### Complexity: Medium

- Cron parser: ~150 LOC (or import `robfig/cron/v3` parser)
- Scheduler goroutine: ~200 LOC
- Event matcher: ~80 LOC
- Skill TriggerConfig extension: ~30 LOC
- Gateway wiring + CLI: ~100 LOC
- Tests: ~200 LOC

**Total estimate:** ~750 LOC, 8 files (5 new, 3 edit)

### Risks

- **Runaway triggers:** Event A triggers skill B which emits event C which
  triggers skill D which emits event A → infinite loop. **Mitigation:** Cooldown
  per entry (minimum interval between runs, e.g. 60s). Events emitted by
  scheduled tasks carry a `source: "scheduler"` marker — the matcher ignores
  events from its own source.

- **Missed ticks:** If the gateway is down during a cron window, the trigger is
  missed. No catch-up. This is acceptable for a personal agent.

---

## F12: Long-term Memory (Knowledge Store)

### The Problem

Every session starts from zero. Ozzie has session history and context compression
(`internal/agent/compressor.go`), but nothing persists _across_ sessions. The
user must re-explain project conventions, preferences, and context every time.

The `PromptComposer` (Layer 5 — `TaskInstructions`) has a stub for injecting
context but no source of persistent knowledge to inject.

**Consequence:** No learning curve. Repeated mistakes. Cannot develop expertise
in the user's domain.

### What Exists

| Component | State | Location |
|-----------|-------|----------|
| Session store | Per-session persistence, no cross-session | `internal/sessions/` |
| Context compression | Summarization within a session | `internal/agent/compressor.go` |
| PromptComposer Layer 5 | Stub (`TaskInstructions`) | `internal/agent/prompt.go:19` |
| Session metadata | `Metadata map[string]string` field | `internal/sessions/session.go:38` |
| OZZIE_PATH | Single root for all data | `internal/config/` |

### Design

#### Data Model

Memories are structured facts stored in `$OZZIE_PATH/memory/`:

```
~/.ozzie/memory/
  ├── index.json        # array of MemoryEntry metadata
  └── entries/
      ├── mem_abc123.md  # content (markdown, can be long)
      └── mem_def456.md
```

```go
type MemoryType string

const (
    MemoryPreference MemoryType = "preference"   // "use tabs", "prefer sonnet"
    MemoryFact       MemoryType = "fact"          // "project X uses clean arch"
    MemoryProcedure  MemoryType = "procedure"     // "to deploy: run make deploy"
    MemoryContext     MemoryType = "context"       // "user is working on project Y"
)

type MemoryEntry struct {
    ID         string     `json:"id"`             // "mem_" + uuid[:8]
    Type       MemoryType `json:"type"`
    Tags       []string   `json:"tags"`           // ["project:ozzie", "lang:go"]
    Title      string     `json:"title"`          // short summary
    CreatedAt  time.Time  `json:"created_at"`
    UpdatedAt  time.Time  `json:"updated_at"`
    LastUsedAt time.Time  `json:"last_used_at"`   // for relevance decay
    Source     string     `json:"source"`          // "user", "task", "inferred"
    Confidence float64    `json:"confidence"`      // 0.0 – 1.0
}
```

Content lives in separate `.md` files to keep the index small and scannable.
This mirrors the session FileStore pattern (meta.json + data files).

#### Retrieval

On each turn, the PromptComposer injects relevant memories into the system
prompt. Retrieval is keyword-based (no embeddings in V1):

1. Extract keywords from the user's message (simple: split + deduplicate)
2. Score memories by tag match + title match + recency
3. Inject top-N (default: 5) into a new Layer 6: `## Relevant Memories`

This avoids embedding model dependencies. Embedding-based retrieval can be added
later as an optimization.

```go
type MemoryStore interface {
    Create(entry *MemoryEntry, content string) error
    Get(id string) (*MemoryEntry, string, error)     // entry + content
    Update(entry *MemoryEntry, content string) error
    Delete(id string) error
    List() ([]*MemoryEntry, error)
    Search(query string, limit int) ([]*MemoryEntry, error)  // keyword search
}
```

#### Retriever Integration

```go
type MemoryRetriever struct {
    store MemoryStore
}

// Retrieve returns memories relevant to the given query, scored and ranked.
func (r *MemoryRetriever) Retrieve(query string, tags []string, limit int) []*MemoryEntry
```

Scoring formula (simple, tunable):
```
score = (tag_match_count * 3) + (title_word_match_count * 2) + recency_bonus
recency_bonus = 1.0 if used in last 7 days, 0.5 if last 30, 0.0 otherwise
```

#### Memory Write Paths

Memories are created via three paths:

1. **Explicit (user says "remember this"):** Agent calls `store_memory` tool
2. **Post-task (automatic):** At task completion, inject a "reflection" prompt
   asking the LLM what was learned. Store high-confidence results.
3. **Inferred (future):** Pattern detection across sessions (e.g., user always
   corrects the same thing). Not in V1.

Path 3 is out of scope for Phase 2b. Path 2 depends on F13 (verify loop).

#### Native Tools

**`store_memory`:**
- Params: `type` (enum), `title` (required), `content` (required), `tags` (optional)
- Writes to memory store
- Returns memory ID

**`query_memories`:**
- Params: `query` (required), `tags` (optional filter), `limit` (default 5)
- Returns matching memories with content snippets

**`forget_memory`:**
- Params: `id` (required)
- Deletes from store

#### PromptComposer Integration

Add Layer 6 after Layer 5:

```go
// Layer 6: Relevant memories
if len(pctx.Memories) > 0 {
    var sb strings.Builder
    sb.WriteString("## Relevant Memories\n\n")
    sb.WriteString("These are things you've learned from previous interactions:\n\n")
    for _, m := range pctx.Memories {
        sb.WriteString(fmt.Sprintf("- [%s] **%s**: %s\n", m.Type, m.Title, m.Content))
    }
    sections = append(sections, sb.String())
}
```

The `EventRunner` calls `retriever.Retrieve(userMessage)` before prompt
composition and passes results into `PromptContext`.

### Files

| File | Action | Notes |
|------|--------|-------|
| `internal/memory/memory.go` | Create | Types: MemoryEntry, MemoryType |
| `internal/memory/store.go` | Create | Store interface |
| `internal/memory/filestore.go` | Create | File-based store (index.json + entries/*.md) |
| `internal/memory/retriever.go` | Create | Keyword-based retrieval + scoring |
| `internal/memory/filestore_test.go` | Create | CRUD, search tests |
| `internal/memory/retriever_test.go` | Create | Scoring, ranking tests |
| `internal/plugins/native_memory.go` | Create | store_memory, query_memories, forget_memory |
| `internal/agent/prompt.go` | Edit | Add Layer 6 (Memories) to PromptContext + Compose |
| `internal/agent/eventrunner.go` | Edit | Call retriever before composition |
| `cmd/commands/gateway.go` | Edit | Wire memory store + tools |
| `cmd/commands/memory.go` | Create | `ozzie memory list/search/forget` |
| `cmd/commands/root.go` | Edit | Add NewMemoryCommand |

### Complexity: Medium

- Store + FileStore: ~300 LOC
- Retriever: ~150 LOC
- Native tools: ~200 LOC
- PromptComposer extension: ~30 LOC
- EventRunner integration: ~20 LOC
- CLI: ~100 LOC
- Tests: ~300 LOC

**Total estimate:** ~1100 LOC, 12 files (8 new, 4 edit)

### Risks

- **Context bloat:** Injecting too many memories inflates the system prompt.
  **Mitigation:** Hard limit (5 memories, max 200 tokens each). Truncate content
  in prompt injection. Full content available via `query_memories` tool.

- **Stale memories:** Old facts become wrong. **Mitigation:** `confidence` field
  + `last_used_at` for decay. `forget_memory` tool for explicit removal. Future:
  periodic review skill that validates old memories.

- **Privacy:** Memories may contain sensitive info. **Mitigation:** All data is
  local (`$OZZIE_PATH/memory/`). No cloud sync. User can inspect/delete freely.

---

## F13: Plan-Execute-Verify Loop

### The Problem

The current skill workflow runner (`internal/skills/runner.go`) is fire-and-
forget: it runs each step, collects the output, moves to the next. The
`acceptance` field exists on Step (`skill.go:54`) and is injected into the step
instruction (`runner.go:220-223`), but:

1. The LLM sees the acceptance criteria as text — nothing enforces them
2. No automated verification that criteria are met
3. No retry loop if output is inadequate
4. No escalation path if retries fail

The task runner (`internal/tasks/runner.go`) has `MaxRetries` and `RetryCount`
fields on Task, but they're only used for infrastructure errors (agent creation
failure), not for quality verification.

**Consequence:** Unattended tasks cannot be trusted. The user must manually
review every output.

### What Exists

| Component | State | Location |
|-----------|-------|----------|
| Step `Acceptance` field | String, injected as text | `internal/skills/skill.go:54` |
| Acceptance in prompt | Appended to step instruction | `internal/skills/runner.go:220-223` |
| Task `MaxRetries` / `RetryCount` | Infrastructure retry only | `internal/tasks/task.go` |
| Task checkpoints | Append-only JSONL | `internal/tasks/filestore.go` |

### Design

#### Verification Model

After each step completes, an optional **verify phase** checks the output
against structured acceptance criteria. Verification uses a separate (cheap)
LLM call with a focused prompt.

```
Execute Step → Verify Output → Pass? → Next Step
                    │
                    ▼ Fail
              Retry with Feedback ← (up to max_attempts)
                    │
                    ▼ Max retries exceeded
              Mark step as "needs_review"
              Continue with degraded output
```

This is NOT a separate agent. It's a focused LLM call (no tools) that returns
a structured verdict:

```go
type VerifyResult struct {
    Pass     bool     `json:"pass"`
    Issues   []string `json:"issues"`   // what's wrong
    Score    int      `json:"score"`    // 0-100
    Feedback string   `json:"feedback"` // instruction for retry
}
```

#### Skill Schema Extension

Extend `Step.Acceptance` from a plain string to a structured object:

```go
// AcceptanceCriteria defines how a step's output is verified.
type AcceptanceCriteria struct {
    Criteria    []string `json:"criteria"`     // individual pass/fail checks
    MaxAttempts int      `json:"max_attempts"` // default: 2 (1 + 1 retry)
    Model       string   `json:"model"`        // verifier model (default: haiku)
}
```

For backward compatibility, if `acceptance` is a plain string in JSONC, it's
parsed as `AcceptanceCriteria{Criteria: []string{s}, MaxAttempts: 2}`.

JSONC example:
```jsonc
{
    "id": "implement",
    "title": "Write the code",
    "instruction": "...",
    "acceptance": {
        "criteria": [
            "Code compiles without errors",
            "All existing tests still pass",
            "New functionality has at least one test"
        ],
        "max_attempts": 3
    }
}
```

#### Verifier

```go
type Verifier struct {
    models *models.Registry
}

// Verify checks step output against acceptance criteria.
// Returns a VerifyResult with pass/fail and detailed feedback.
func (v *Verifier) Verify(ctx context.Context, criteria AcceptanceCriteria,
    stepTitle string, output string) (*VerifyResult, error)
```

The verifier builds a focused prompt:

```
You are a quality verifier. Evaluate the following output against the criteria.

## Step: {stepTitle}

## Output to verify:
{output}

## Criteria:
1. {criteria[0]}
2. {criteria[1]}
...

Respond with JSON: {"pass": bool, "issues": [...], "score": 0-100, "feedback": "..."}
```

Uses a cheap model (haiku by default) to minimize cost. Parse the JSON from the
LLM response (with fallback: if parsing fails, treat as pass with warning).

#### Integration with WorkflowRunner

Modify `WorkflowRunner.runStep()`:

```go
func (wr *WorkflowRunner) runStep(ctx, stepID, vars, prevResults) (string, error) {
    step := wr.dag.Step(stepID)
    // ... existing code: resolve model, tools, build instruction, run agent ...
    output, err := consumeRunnerOutput(iter)
    if err != nil { return "", err }

    // NEW: Verify if acceptance criteria exist
    if step.Acceptance != nil && len(step.Acceptance.Criteria) > 0 {
        output, err = wr.verifyAndRetry(ctx, step, output, vars, prevResults)
    }

    return output, err
}

func (wr *WorkflowRunner) verifyAndRetry(ctx, step, output, vars, prevResults) (string, error) {
    maxAttempts := step.Acceptance.MaxAttempts
    if maxAttempts <= 0 { maxAttempts = 2 }

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        result, err := wr.verifier.Verify(ctx, *step.Acceptance, step.Title, output)
        if err != nil {
            slog.Warn("verification failed", "error", err)
            break // verification error → accept output as-is
        }

        // Emit verification event
        wr.emitVerification(step, result, attempt)

        if result.Pass {
            return output, nil
        }

        if attempt == maxAttempts {
            slog.Warn("step failed verification after max attempts",
                "step", step.ID, "issues", result.Issues)
            return output, nil // degraded: return last output
        }

        // Retry with feedback
        output, err = wr.retryStep(ctx, step, output, result.Feedback, vars, prevResults)
        if err != nil {
            return "", err
        }
    }
    return output, nil
}
```

#### Integration with TaskRunner

The task runner (`internal/tasks/runner.go`) should also support verification
when a task has a plan with acceptance criteria. Since `TaskRunner.runPlanSteps`
already iterates over `TaskPlanStep`, we add verification there too.

However, `TaskPlanStep` currently has no acceptance criteria. For Phase 2b, the
task runner delegates to the skill runner for skill-based tasks. Standalone tasks
(submitted via `submit_task`) don't have acceptance criteria yet — that's a
Phase 2c enhancement.

#### Events

New event types (add to `internal/events/bus.go`):

```go
EventTaskVerification EventType = "task.verification"
```

Payload:
```go
type TaskVerificationPayload struct {
    TaskID    string   `json:"task_id,omitempty"`
    SkillName string   `json:"skill_name,omitempty"`
    StepID    string   `json:"step_id"`
    Pass      bool     `json:"pass"`
    Score     int      `json:"score"`
    Issues    []string `json:"issues,omitempty"`
    Attempt   int      `json:"attempt"`
}
```

### Files

| File | Action | Notes |
|------|--------|-------|
| `internal/skills/acceptance.go` | Create | AcceptanceCriteria type, backward compat parser |
| `internal/skills/verifier.go` | Create | Verifier struct, Verify method |
| `internal/skills/verifier_test.go` | Create | Tests with mock LLM |
| `internal/skills/skill.go` | Edit | Step.Acceptance → AcceptanceCriteria (with compat) |
| `internal/skills/runner.go` | Edit | Add verifyAndRetry loop |
| `internal/events/bus.go` | Edit | Add EventTaskVerification |
| `internal/events/payloads.go` | Edit | Add TaskVerificationPayload |

### Complexity: High

- Acceptance types + compat parser: ~100 LOC
- Verifier: ~150 LOC
- Runner retry loop: ~100 LOC
- Events: ~30 LOC
- Tests: ~250 LOC

**Total estimate:** ~630 LOC, 7 files (3 new, 4 edit)

### Risks

- **Cost amplification:** Each verification is an LLM call. A 5-step workflow
  with 3 retries each = 5 + 15 = 20 extra calls. **Mitigation:** Use cheapest
  model (haiku). Verification is optional (only when `acceptance` is defined).
  Default `max_attempts: 2` limits total calls.

- **Verification hallucination:** The verifier LLM may incorrectly pass or fail.
  **Mitigation:** Structured JSON output with `issues` field — the retry step
  gets specific feedback. Score threshold tunable (default: pass if score >= 70).

- **Backward compatibility:** Changing `Step.Acceptance` from `string` to
  `AcceptanceCriteria` breaks existing JSONC. **Mitigation:** Custom
  `UnmarshalJSON` that accepts both string and object forms.

---

## Implementation Order

Dependencies dictate the order:

```
F11: Scheduler ─────────── FIRST
  │                        Enables time-based autonomy.
  │                        No dependencies on F12/F13.
  │
  ▼
F12: Long-term Memory ──── SECOND
  │                        Uses scheduler for future memory maintenance.
  │                        Enables F13 to store learned patterns.
  │
  ▼
F13: Plan-Execute-Verify ─ THIRD
  │                        Can store verification patterns in memory.
  │                        Most complex, benefits from stable F11+F12.
```

Each foundation is independently useful. F11 alone enables cron jobs. F12 alone
enables cross-session learning. F13 alone enables self-correction. But together
they form a feedback loop:

```
Schedule task → Execute → Verify → Learn from result → Remember for next time
     ▲                                                        │
     └────────────────────────────────────────────────────────┘
```

---

## Summary

| Foundation | New Files | Edited Files | LOC Estimate | Complexity |
|-----------|-----------|--------------|-------------|------------|
| F11: Scheduler | 5 | 3 | ~750 | Medium |
| F12: Memory | 8 | 4 | ~1100 | Medium |
| F13: Verify | 3 | 4 | ~630 | High |
| **Total** | **16** | **11** | **~2480** | — |

### Quality Gates

Every milestone must pass:
```bash
go build ./...
~/go/bin/staticcheck ./...
go test ./...
```

### What This Enables (Phase 2c)

With F11+F12+F13 in place:

- **Self-engineering loop:** Schedule a code review skill → verify quality →
  remember what patterns work → iterate
- **Proactive maintenance:** Cron-triggered skills for dependency updates,
  security scanning, documentation freshness
- **Learning agent:** Ozzie remembers project conventions, user preferences,
  and successful approaches across sessions
- **Trusted autonomy:** Verification loop means unattended tasks produce
  reliable output without manual review
