# Plan V1 — Foundations

> Insights from Gastown applied to Ozzie.
> See `docs/references/gastown-analysis.md` for the full Gastown reference.

## Current State (Phase 1a — done)

Working E2E: `ozzie gateway` -> `ozzie ask "hello"` -> streamed LLM response.

**What exists:**
- Event bus (in-memory, ring buffer, typed payloads)
- Model registry (Anthropic, OpenAI, Ollama)
- Gateway + WebSocket hub
- Agent (Eino ADK + EventRunner + ReAct loop)
- Plugin system (WASM Extism + native tools + manifests)
- MCP server
- Config (JSONC + env templating)

**What's stubbed (doc.go only):**
- `internal/sessions/` — session management
- `internal/storage/` — persistent storage
- `internal/scheduler/` — task scheduling
- `clients/tui/` — terminal UI

---

## Foundation 1: Session Persistence

**Problem:** Conversation history lives only in `EventRunner.messages` (in-memory).
Gateway restart = total amnesia. No crash recovery. No session listing.

**Gastown insight:** All state in Git-friendly files. "Zero Framework Cognition" —
state derived from filesystem, never from memory cache.

### Design

Sessions stored as JSONL files under `$OZZIE_PATH/sessions/`:

```
~/.ozzie/sessions/
  ├── index.jsonl                    # Session metadata index
  ├── sess_abc123/
  │   ├── meta.json                  # Session metadata
  │   └── messages.jsonl             # Conversation messages (append-only)
  └── sess_def456/
      ├── meta.json
      └── messages.jsonl
```

**Session metadata** (`meta.json`):
```json
{
  "id": "sess_abc123",
  "title": "auto-generated or user-set",
  "created_at": "2026-02-21T10:00:00Z",
  "updated_at": "2026-02-21T10:05:00Z",
  "status": "active",
  "model": "anthropic/claude-sonnet-4-20250514",
  "message_count": 12,
  "token_usage": { "input": 4500, "output": 2100 }
}
```

**Message line** (`messages.jsonl`, one JSON object per line):
```json
{"role":"user","content":"hello","ts":"2026-02-21T10:00:00Z"}
{"role":"assistant","content":"Hi! How can I help?","ts":"2026-02-21T10:00:01Z","tokens_in":10,"tokens_out":8}
{"role":"user","content":"what is 2+2?","ts":"2026-02-21T10:01:00Z"}
{"role":"assistant","content":"4","ts":"2026-02-21T10:01:01Z","tokens_in":25,"tokens_out":3,"tool_calls":[{"name":"calculator","args":{"expr":"2+2"},"result":"4"}]}
```

### Changes

| Package | What |
|---------|------|
| `internal/sessions/` | `Session` struct, `Store` interface, `FileStore` (JSONL-backed) |
| `internal/sessions/` | `Manager` — create, list, get, restore, archive sessions |
| `internal/agent/eventrunner.go` | Inject `sessions.Manager`, persist messages on each turn |
| `internal/agent/eventrunner.go` | On startup, restore last active session's messages |
| `internal/config/config.go` | Add `SessionsConfig` (dir, max_history, auto_title) |
| `cmd/commands/sessions.go` | Implement `ozzie sessions list/show/resume` (currently stub) |
| `cmd/commands/gateway.go` | Wire session manager into gateway startup |
| `internal/events/types.go` | Already has `EventSessionCreated`/`EventSessionClosed` — use them |

### Key Decisions

- **JSONL not SQLite** — Git-friendly, human-readable, appendable. SQLite comes
  later (event store) per brainstorming-v2 ADR 004.
- **Append-only messages** — Never rewrite history. Compaction (summarize old
  messages) can be a later feature.
- **Session per WS connection** — New WS connection = new or resumed session.
  Session ID passed in WS handshake query params.

---

## Foundation 2: Declarative Workflows (Skills as DAGs)

**Problem:** Skills are planned as JSONC files (brainstorming-v2 section 5), but
only as single-agent configs. No multi-step workflows, no dependencies, no
acceptance criteria.

**Gastown insight:** Formulas are TOML DAGs with steps, dependencies, variables,
and acceptance criteria. Molecules are persistent runtime instances. This is the
foundation for both skills and the self-engineering loop.

### Design

Extend the existing skill JSONC format with workflow support. Two skill types:

**Type 1: Simple skill** (existing plan, single agent):
```jsonc
{
  "name": "researcher",
  "description": "Deep web research and synthesis",
  "model": "sonnet",
  "instruction": "You are a research specialist...",
  "tools": ["web_search", "fetch_url"],
  "triggers": { "delegation": true }
}
```

**Type 2: Workflow skill** (new, DAG of steps):
```jsonc
{
  "name": "code-review",
  "description": "Automated code review with multiple review legs",
  "type": "workflow",
  "version": 1,

  // Input variables (user must provide or defaults apply)
  "vars": {
    "target": {
      "description": "Branch, PR number, or file paths to review",
      "required": true
    },
    "focus": {
      "description": "Review focus areas",
      "default": "all"
    }
  },

  // DAG steps
  "steps": [
    {
      "id": "gather-context",
      "title": "Gather code context",
      "instruction": "Read the changed files and understand the diff",
      "tools": ["cmd", "file_read"],
      "model": "haiku"
    },
    {
      "id": "review-correctness",
      "title": "Review correctness",
      "instruction": "Check for logical errors, edge cases, off-by-ones",
      "needs": ["gather-context"],
      "model": "sonnet"
    },
    {
      "id": "review-security",
      "title": "Review security",
      "instruction": "Check for OWASP top 10, injection, auth issues",
      "needs": ["gather-context"],
      "model": "sonnet"
    },
    {
      "id": "review-style",
      "title": "Review Go style",
      "instruction": "Check gofmt, staticcheck, naming conventions",
      "needs": ["gather-context"],
      "model": "haiku"
    },
    {
      "id": "synthesize",
      "title": "Synthesize review",
      "instruction": "Combine all reviews into a final assessment with scores",
      "needs": ["review-correctness", "review-security", "review-style"],
      "model": "sonnet",
      "acceptance": "All critical issues addressed"
    }
  ]
}
```

### Workflow Engine Design

```
                SkillLoader
                    |
        +-----------+-----------+
        |                       |
   SimpleSkill            WorkflowSkill
   (single agent)         (DAG engine)
                               |
                          WorkflowRunner
                               |
                    +----------+----------+
                    |          |          |
                StepRunner StepRunner StepRunner
                (gather)   (review-1) (review-2)
                    |          |          |
                    +----------+----------+
                               |
                          StepRunner
                          (synthesize)
```

### Changes

| Package | What |
|---------|------|
| `internal/skills/` | `Skill` struct (simple + workflow types), `Step` struct |
| `internal/skills/` | `Loader` — reads `$OZZIE_PATH/skills/*.jsonc` + built-in skills |
| `internal/skills/` | `DAG` — topological sort, ReadySteps(completed), cycle detection |
| `internal/skills/` | `WorkflowRunner` — executes DAG, tracks step completion, emits events |
| `internal/skills/` | `StepRunner` — executes single step (creates ephemeral agent + tools) |
| `internal/events/types.go` | Add: `skill.started`, `skill.step.started`, `skill.step.completed`, `skill.completed` |
| `internal/events/payloads.go` | Add: `SkillPayload`, `SkillStepPayload` |
| `internal/config/config.go` | Add `SkillsConfig` (dirs: []string for multiple skill directories) |
| `cmd/commands/gateway.go` | Wire skill loader + register workflow tools |

### Key Decisions

- **JSONC not TOML** — Consistent with existing config format. JSONC is already
  a dependency. Gastown uses TOML, but we prefer consistency.
- **Steps are independent agents** — Each step creates its own ephemeral Eino
  agent with its own model, tools, and instruction. This enables per-step model
  selection (haiku for simple tasks, sonnet for complex).
- **Steps emit events** — Each step start/complete/fail goes through the event
  bus. This means the WS hub broadcasts progress to connected clients.
- **No molecules yet** — Gastown's molecule system (persistent workflow instances)
  is Phase 2. For now, workflow execution is synchronous within a session.

---

## Foundation 3: Dynamic Prompt Composition

**Problem:** `DefaultSystemPrompt` in `internal/agent/agent.go` is a static
string. Can't adapt to context (active tools, session state, skill being
executed).

**Gastown insight:** Priming is multi-layered and state-aware. Context is
detected, not assumed. Injection adapts to startup mode (normal, resume, crash).

### Design

Enrich the persona prompt with dynamic context layers. The persona
(`DefaultSystemPrompt`) is the **immutable foundation** — Ozzie's identity, tone,
and personality. The composer appends contextual layers on top, never replaces it.

```
┌─────────────────────────────────┐
│  1. PERSONA (immutable)         │  <- DefaultSystemPrompt: Ozzie's identity, personality,
│                                 │     tone, core rules. This is WHO Ozzie is.
│                                 │     Always present, never overridden by context.
├─────────────────────────────────┤
│  2. Active Tools Manifest       │  <- "You have access to: calculator, web_search..."
├─────────────────────────────────┤
│  3. Active Skills Manifest      │  <- "You can delegate to: researcher, code-review..."
├─────────────────────────────────┤
│  4. Session Context             │  <- "Resumed session. Previous topic: ..."
├─────────────────────────────────┤
│  5. Task-Specific Instructions  │  <- Only if executing a workflow step
└─────────────────────────────────┘
```

Ozzie remains Ozzie in every context. The dynamic layers tell him what tools he
has and what he's working on — they don't change who he is.

### Changes

| Package | What |
|---------|------|
| `internal/agent/prompt.go` | New file: `PromptComposer` with `Compose(ctx PromptContext) string` |
| `internal/agent/prompt.go` | `PromptContext` struct: tools, skills, session, task (persona is always `DefaultSystemPrompt`) |
| `internal/agent/agent.go` | Use `PromptComposer` instead of passing raw `DefaultSystemPrompt` |
| `internal/agent/eventrunner.go` | Pass current context to composer on each turn |
| `internal/plugins/registry.go` | Add `ToolManifest() string` method for prompt injection |

### Key Decisions

- **Persona is sacred** — `DefaultSystemPrompt` is always layer 1, untouched.
  The user can extend it via config (`agent.system_prompt`) but never replace it.
  Ozzie's personality persists across all contexts, tools, and workflow steps.
- **Recompose per turn** — Not just at startup. As tools/skills change
  (hot-reload), the prompt adapts. Cost: one string concat per turn (negligible).
- **No templating engine** — Simple Go string concatenation. No text/template
  complexity. Layers are plain strings appended after the persona.

---

## Foundation 4: Event Persistence (Lightweight)

**Problem:** Event bus is in-memory only. Ring buffer (last N events) is the only
history. No crash recovery, no replay, no audit trail.

**Gastown insight:** Everything is persisted in Git-friendly formats. Beads are
JSONL. Work survives everything.

### Design

Add a lightweight event persistence layer that writes to JSONL files. NOT the
full SQLite event store from brainstorming-v2 — that's Phase 2. This is the
minimal "don't lose data" layer.

```
~/.ozzie/events/
  ├── 2026-02-21.jsonl    # One file per day, append-only
  └── 2026-02-22.jsonl
```

Each line is a serialized `events.Event`:
```json
{"id":"evt_abc","session_id":"sess_123","type":"assistant.message","ts":"2026-02-21T10:00:01Z","source":"agent","payload":{"content":"Hello!"}}
```

### Changes

| Package | What |
|---------|------|
| `internal/storage/` | `EventLog` — append-only JSONL writer, daily rotation |
| `internal/storage/` | `EventReader` — read events by date range, type filter |
| `cmd/commands/gateway.go` | Subscribe EventLog to the bus (write all events) |
| `internal/events/bus.go` | No changes needed — EventLog is just another subscriber |

### Key Decisions

- **JSONL not SQLite** — Phase 1 stays file-based for simplicity and
  Git-friendliness. SQLite event store is Phase 2.
- **Daily rotation** — One file per day prevents unbounded growth. Archival
  (compress old files) is a later feature.
- **Write-only for now** — We persist events but don't read them back yet
  (except for debugging). Replay/projection is Phase 2.
- **No performance concern** — At personal agent scale (~100-1000 events/day),
  JSONL append is effectively free.

---

## Foundation 5: Cost Tracking

**Problem:** `LLMCallPayload` already carries token counts and model info, but
this data is lost when the event leaves the ring buffer.

**Gastown insight:** Every session records costs. The `gt costs record` hook runs
on session stop. Historical data enables model comparison and optimization.

### Design

Aggregate cost data from `LLMCallPayload` events into session metadata.

### Changes

| Package | What |
|---------|------|
| `internal/sessions/` | Add `TokenUsage` and `CostEstimate` to session metadata |
| `internal/agent/eventrunner.go` | On LLMCall events, update session token counters |
| `internal/models/` | Add `EstimateCost(provider, tokensIn, tokensOut) float64` |
| `cmd/commands/status.go` | Show cost summary in `ozzie status` |

### Key Decisions

- **Estimate, not exact** — We use published pricing tiers. Close enough for
  personal tracking. Exact billing comes from the provider dashboard.
- **Per-session aggregation** — Total tokens in/out, estimated cost, model used.
  Stored in session `meta.json`.
- **No external reporting** — Just local data. The user can inspect
  `sessions/*/meta.json` to see costs.

---

## Implementation Order

Foundations are ordered by dependency. Each builds on the previous:

```
F1: Session Persistence ──────── FIRST (EventRunner needs it)
  |
  v
F3: Dynamic Prompt Composition ─ SECOND (uses session context)
  |
  v
F4: Event Persistence ────────── THIRD (just a bus subscriber)
  |
  v
F5: Cost Tracking ────────────── FOURTH (uses F1 sessions + F4 events)
  |
  v
F2: Declarative Workflows ────── LAST (most complex, uses all above)
```

**Estimated scope per foundation:**

| Foundation | New files | Modified files | Complexity |
|------------|-----------|----------------|------------|
| F1: Sessions | 3-4 | 3 | Medium |
| F3: Prompt Composer | 1-2 | 2 | Low |
| F4: Event Persistence | 2 | 1 | Low |
| F5: Cost Tracking | 1 | 3 | Low |
| F2: Workflows | 5-6 | 3 | High |

---

## What This Enables (Post-V1)

Once these foundations are in place, the self-engineering loop becomes a natural
extension:

1. **Workflow skill** `self-engineer.jsonc` defines the coding loop
2. **Session persistence** ensures work survives interruptions
3. **Event persistence** provides full audit trail of what the agent did
4. **Cost tracking** shows how much each self-engineering session costs
5. **Dynamic prompts** adapt the agent's instructions per workflow step

The remaining pieces for self-engineering (worker isolation via worktrees, merge
queue, attribution) become straightforward additions on top of these foundations.

---

## References

- Gastown analysis: `docs/references/gastown-analysis.md`
- Ozzie brainstorming-v2: `docs/brainstorming-v2.md`
- Current architecture: `CLAUDE.md`
- Quality gates: `.claude/rules/go-quality.md`
