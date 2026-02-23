# Gastown Analysis — Reference for Ozzie

> **Source:** <https://github.com/steveyegge/gastown>
> **Author:** Steve Yegge (ex-Google, ex-Amazon, Sourcegraph)
> **Language:** Go 1.23+, Cobra CLI, ~150 commands, 40+ internal packages
> **Date of analysis:** 2026-02-21

## Table of Contents

- [1. What Gastown Is](#1-what-gastown-is)
- [2. Key Concepts Worth Adopting](#2-key-concepts-worth-adopting)
  - [2.1 Git as State Backbone](#21-git-as-state-backbone)
  - [2.2 Declarative Workflows (Formulas)](#22-declarative-workflows-formulas)
  - [2.3 Dynamic Prompt Composition (Priming)](#23-dynamic-prompt-composition-priming)
  - [2.4 Durable Hooks](#24-durable-hooks)
  - [2.5 Plugin System (Markdown + TOML)](#25-plugin-system-markdown--toml)
  - [2.6 Inter-Agent Mail](#26-inter-agent-mail)
  - [2.7 Cost Tracking](#27-cost-tracking)
- [3. Multi-Agent Architecture](#3-multi-agent-architecture)
- [4. Self-Engineering Loop](#4-self-engineering-loop)
- [5. What NOT to Copy](#5-what-not-to-copy)
- [6. Key Source Files Reference](#6-key-source-files-reference)
- [7. Glossary](#7-glossary)

---

## 1. What Gastown Is

Gastown is a **multi-agent orchestration platform** for Claude Code. It manages
fleets of AI agents working on codebases, using Git as the persistence backbone
and tmux for agent lifecycle management.

Core philosophy: **"Zero Framework Cognition" (ZFC)** — agents decide, Go
transports. State is derived from the filesystem, never cached in memory.

Architecture: two-level beads (issue tracking backed by Git/Dolt):
- **Town level** (`~/gt/.beads/`) — cross-rig coordination, Mayor mail, agent identity
- **Rig level** (`<rig>/mayor/rig/.beads/`) — implementation work, MRs, project issues

---

## 2. Key Concepts Worth Adopting

### 2.1 Git as State Backbone

**Concept:** All work state lives in Git. Beads (issues/tasks) are JSONL files
tracked in Git repositories. Work survives crashes, compaction, and handoffs.

**Gastown implementation:**
- Beads = JSONL issue tracker backed by Dolt (Git-compatible SQL database)
- Each rig gets its own Dolt database under `~/gt/.dolt-data/`
- Write concurrency: each worker gets its own branch (`BD_BRANCH` env var),
  merged to main at completion
- Routing: `routes.jsonl` maps bead ID prefixes to rig paths

**Bead struct** (simplified):
```go
type Issue struct {
    ID          string   `json:"id"`          // <prefix>-<5-char-alphanum>
    Title       string   `json:"title"`
    Description string   `json:"description"`
    Status      string   `json:"status"`      // open, in_progress, hooked, closed
    Type        string   `json:"type"`        // task, bug, feature, epic, molecule, message
    Priority    int      `json:"priority"`    // 0=critical ... 4=backlog
    Assignee    string   `json:"assignee"`
    Labels      []string `json:"labels"`
    Parent      string   `json:"parent,omitempty"`
    DependsOn   []string `json:"depends_on,omitempty"`
    HookBead    string   `json:"hook_bead,omitempty"`
    CreatedAt   string   `json:"created_at,omitempty"`
    UpdatedAt   string   `json:"updated_at,omitempty"`
}
```

**Relevance for Ozzie:** We don't need Dolt — plain JSONL files in
`$OZZIE_PATH/sessions/` and `$OZZIE_PATH/tasks/` would give us crash recovery
and auditability. Git-friendly formats (JSONL, Markdown) mean the user can
inspect state with any text editor.

### 2.2 Declarative Workflows (Formulas)

**Concept:** Multi-step workflows defined as TOML files with full DAG support.
Four formula types: convoy (parallel legs), workflow (sequential steps),
expansion (template-based), aspect (multi-aspect analysis).

**Source files:**
- `internal/formula/types.go` — Core structs
- `internal/formula/parser.go` — TOML parser with type inference
- `internal/formula/embed.go` — Embedded formulas via `//go:embed`
- `internal/formula/formulas/*.formula.toml` — 28 embedded formulas

**Core structs:**
```go
type Formula struct {
    Name        string      `toml:"formula"`
    Description string      `toml:"description"`
    Type        FormulaType `toml:"type"`       // convoy, workflow, expansion, aspect
    Version     int         `toml:"version"`
    Steps       []Step      `toml:"steps"`      // Workflow steps
    Vars        map[string]Var `toml:"vars"`    // Input variables
    Legs        []Leg       `toml:"legs"`       // Convoy parallel legs
    Synthesis   *Synthesis  `toml:"synthesis"`  // Convoy synthesis step
}

type Step struct {
    ID          string   `toml:"id"`
    Title       string   `toml:"title"`
    Description string   `toml:"description"`
    Needs       []string `toml:"needs"`       // DAG dependencies
    Parallel    bool     `toml:"parallel"`
    Acceptance  string   `toml:"acceptance"`  // Exit criteria
}

type Var struct {
    Description string `toml:"description"`
    Required    bool   `toml:"required"`
    Default     string `toml:"default"`
}
```

**DAG support:** Topological sort (Kahn's algorithm), `ReadySteps(completed)`,
cycle detection, dependency resolution.

**Example — code-review formula (convoy type):**
```toml
formula = "code-review"
type = "convoy"
version = 1

[inputs.pr]
description = "Pull request number"
type = "number"
required_unless = ["files", "branch"]

[[legs]]
id = "correctness"
title = "Correctness Review"
focus = "Logical correctness and edge case handling"

[[legs]]
id = "performance"
title = "Performance Review"
focus = "Performance implications and optimizations"

[[legs]]
id = "security"
title = "Security Review"
focus = "Security vulnerabilities and best practices"

# ... 7 more legs

[synthesis]
title = "Review Synthesis"
depends_on = ["correctness", "performance", "security", ...]
```

**Example — polecat work formula (workflow type):**
```toml
formula = "mol-polecat-work"
type = "workflow"
version = 1

[vars.issue]
description = "The issue bead ID to work on"
required = true

[vars.test_command]
description = "Command to run tests"
default = "go test ./..."

[[steps]]
id = "load-context"
title = "Load issue context and requirements"

[[steps]]
id = "branch-setup"
title = "Create feature branch"
needs = ["load-context"]

[[steps]]
id = "preflight-tests"
title = "Run existing tests to verify clean state"
needs = ["branch-setup"]

[[steps]]
id = "implement"
title = "Implement the changes"
needs = ["preflight-tests"]

[[steps]]
id = "self-review"
title = "Review own changes"
needs = ["implement"]

[[steps]]
id = "run-tests"
title = "Run full test suite"
needs = ["self-review"]

[[steps]]
id = "commit-changes"
title = "Commit with attribution"
needs = ["run-tests"]
```

**Molecules** are runtime instances of formulas:
```
Formula (TOML source) --cook--> Protomolecule (frozen template)
    |-> pour --> Molecule (persistent, tracked)
    |-> wisp --> Wisp (ephemeral, auto-cleaned)
```

**Relevance for Ozzie:** This is the pattern for our skills system AND for the
self-engineering loop. A "skill" can be a TOML workflow with steps, deps, and
variables. The engineering loop becomes: `implement -> test -> lint -> review ->
commit`. Our quality gates (`go build`, `staticcheck`, `go test`) become step
acceptance criteria.

### 2.3 Dynamic Prompt Composition (Priming)

**Concept:** System prompt is assembled dynamically based on detected state, not
hardcoded. The `gt prime` command detects context and produces role-specific
instructions.

**Priming modes:**
- **Normal startup** — full context (role identity, capabilities, workflow instructions)
- **Post-handoff** — minimal (agent already has context in compressed memory)
- **Crash recovery** — hooked work + pending mail only
- **Autonomous** — detected work triggers immediate execution

**Multi-layer context injection:**
- `PRIME.md` — global rules injected to all agents
- `CLAUDE.md` / `AGENTS.md` — role-specific templates
- Environment variables — `GT_ROLE`, `GT_RIG`, `BD_ACTOR` for identity
- Mail injection — pending messages after resume

**Source files:**
- `internal/cmd/prime.go` — Prime command logic
- `internal/templates/roles/` — Role-specific prompt templates

**Relevance for Ozzie:** Our `DefaultSystemPrompt` in `internal/agent/agent.go`
is static. We already planned dynamic composition in brainstorming-v2 (section
3.2) but haven't implemented it. The priming pattern — detect state, compose
layers — is the right approach. Layers: persona + active tools manifest +
session context + skill instructions.

### 2.4 Durable Hooks

**Concept:** Work assigned to an agent is "hooked" — attached to the agent's
identity bead. The hook survives session restarts, context compaction, and
handoffs. Combined with GUPP ("If there is work on your Hook, YOU MUST RUN IT"),
this creates autonomous execution without polling.

**Implementation via Claude Code hooks:**
```json
{
    "SessionStart": [{"matcher": "", "hooks": [{"type": "command", "command": "gt prime --hook"}]}],
    "PreCompact": [{"matcher": "", "hooks": [{"type": "command", "command": "gt prime --hook"}]}],
    "UserPromptSubmit": [{"matcher": "", "hooks": [{"type": "command", "command": "gt mail check --inject"}]}],
    "Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "gt costs record"}]}]
}
```

**Source files:**
- `internal/hooks/config.go` — Hook struct definitions
- `internal/hooks/` — Override system (base + role-level + rig-level)

**Relevance for Ozzie:** When Ozzie becomes an orchestrator (self-coding loop),
tasks need to survive gateway restarts. Our event bus is in-memory with a ring
buffer — events are lost on restart. We need a persistence layer for "active
work" that survives process death.

### 2.5 Plugin System (Markdown + TOML)

**Concept:** Plugins are directories with a `plugin.md` file containing TOML
frontmatter and Markdown instructions. Gate system controls execution frequency.

**Plugin struct:**
```go
type Plugin struct {
    Name         string    `json:"name"`
    Description  string    `json:"description"`
    Version      int       `json:"version"`
    Location     Location  `json:"location"`     // "town" or "rig"
    Gate         *Gate     `json:"gate,omitempty"`
    Instructions string    `json:"instructions,omitempty"`
}

type Gate struct {
    Type     GateType `json:"type"`      // cooldown, cron, condition, event, manual
    Duration string   `json:"duration,omitempty"`
    Schedule string   `json:"schedule,omitempty"`
    Check    string   `json:"check,omitempty"`
    On       string   `json:"on,omitempty"`
}
```

**Format:**
```markdown
+++
name = "github-sheriff"
description = "Monitor GitHub CI checks on open PRs"
version = 1

[gate]
type = "cooldown"
duration = "5m"

[tracking]
labels = ["plugin:github-sheriff", "category:ci-monitoring"]
digest = true
+++

# GitHub Sheriff
<agent instructions in markdown>
```

**Relevance for Ozzie:** Our plugin system is already more sophisticated (WASM +
manifests). But the **gate concept** (cooldown, cron, condition, event) is
useful for our scheduler stub. And the `plugin.md` format (TOML frontmatter +
Markdown instructions) is elegant for skills that need rich instructions.

### 2.6 Inter-Agent Mail

**Concept:** Agents communicate via messages stored as beads. Supports direct
addressing, queues (claim-based), and channels (broadcast). Priority levels and
threading.

**Message struct:**
```go
type Message struct {
    ID        string      `json:"id"`
    From      string      `json:"from"`
    To        string      `json:"to"`
    Subject   string      `json:"subject"`
    Body      string      `json:"body"`
    Timestamp time.Time   `json:"timestamp"`
    Priority  Priority    `json:"priority"`    // low, normal, high, urgent
    Type      MessageType `json:"type"`        // task, notification, reply
    Delivery  Delivery    `json:"delivery"`    // queue or interrupt
    ThreadID  string      `json:"thread_id,omitempty"`
}
```

**Protocol messages:**
- `POLECAT_DONE` — Worker -> Witness (work completed)
- `MERGE_READY` — Witness -> Refinery (ready for merge)
- `MERGED` / `MERGE_FAILED` — Refinery -> Witness (result)
- `REWORK_REQUEST` — Refinery -> Witness (conflicts)

**Relevance for Ozzie:** Not needed in V1 (single agent). Essential for the
self-engineering loop where orchestrator and workers need to communicate.
Our event bus already supports typed payloads — extending it with persistent
messages is straightforward.

### 2.7 Cost Tracking

**Concept:** Every session tracks tokens used, USD cost, and model. The `gt
costs record` command runs on session stop (via hook) to persist cost data.
Enables A/B comparison between models on similar tasks.

**Relevance for Ozzie:** Our `LLMCallPayload` already carries `TokensInput`,
`TokensOutput`, `Model`, `Provider`, `Duration`. We just need to persist and
aggregate this data. Low effort, high value for the self-improvement loop.

---

## 3. Multi-Agent Architecture

Gastown defines 6 agent roles:

| Role | Scope | Type | Function |
|------|-------|------|----------|
| **Mayor** | Town | Persistent | Global coordinator, cross-rig orchestration |
| **Deacon** | Town | Persistent | Watchdog, health checks, plugin execution |
| **Witness** | Rig | Persistent | Passive monitor, polecat health, cleanup |
| **Refinery** | Rig | Persistent | Merge queue processor, PR validation |
| **Polecat** | Rig | Ephemeral sessions | Workers with persistent identity |
| **Crew** | Rig | Persistent | Human workspaces, full git clones |

**Lifecycle patterns:**
- Infrastructure agents (Mayor, Deacon, Witness, Refinery) are persistent
- Workers (Polecats) have persistent identity but ephemeral sessions
- Sessions cycle via `gt handoff` (for context compression) or `gt done` (completion)

**Worker isolation:**
- Polecats work in **git worktrees** (not full clones) from `mayor/rig`
- Each polecat gets its own Dolt branch for write isolation
- Branches merged to main at completion

**Relevance for Ozzie's self-engineering loop:**
- We need: Orchestrator (Mayor-like) + Workers (Polecat-like)
- Worktree isolation is critical — each coding task in its own worktree
- Merge queue (Refinery) with quality gates = our `go build + staticcheck + go test`

---

## 4. Self-Engineering Loop

Gastown does NOT modify its own code. It orchestrates agents that code on OTHER
projects. But it provides all the building blocks for a self-engineering loop:

**Available primitives:**
1. Worker isolation (worktrees)
2. Declarative workflows (formulas with DAG)
3. Quality gates as acceptance criteria
4. Attribution (every commit carries agent identity)
5. Cost tracking (tokens, USD per task)
6. Code review formula (10 parallel review legs + synthesis)
7. Merge queue (Refinery validates before merge)

**What Ozzie needs to add for self-coding:**
1. The orchestrator points workers AT ITSELF (Ozzie's own codebase)
2. Quality gates are already defined: `go build`, `staticcheck`, `go test`
3. A "self-improve" formula: analyze errors -> create task -> implement -> test -> review -> merge
4. Safeguards: never merge to main without all gates passing, human approval for architectural changes

**Proposed self-engineering formula:**
```toml
formula = "self-engineer"
type = "workflow"
version = 1

[vars.objective]
description = "What to implement or fix"
required = true

[[steps]]
id = "analyze"
title = "Analyze objective and current codebase"

[[steps]]
id = "plan"
title = "Create implementation plan"
needs = ["analyze"]

[[steps]]
id = "branch"
title = "Create feature branch + worktree"
needs = ["plan"]

[[steps]]
id = "implement"
title = "Write the code"
needs = ["branch"]

[[steps]]
id = "quality-gates"
title = "Run go build && staticcheck && go test"
needs = ["implement"]
acceptance = "All three gates pass with zero errors"

[[steps]]
id = "self-review"
title = "Review own changes for correctness and style"
needs = ["quality-gates"]

[[steps]]
id = "commit"
title = "Commit with attribution"
needs = ["self-review"]

[[steps]]
id = "request-merge"
title = "Create PR or request human approval"
needs = ["commit"]
```

---

## 5. What NOT to Copy

| Gastown Feature | Why Not for Ozzie |
|-----------------|-------------------|
| **Dolt SQL server** | Overkill for personal agent. Plain JSONL + SQLite is enough |
| **6 agent roles** | Enterprise complexity. Ozzie needs 2: orchestrator + worker |
| **tmux-based lifecycle** | Fragile coupling. Ozzie's gateway pattern is better |
| **Beads ID routing** | Multi-rig feature we don't need |
| **Dogs (cross-rig helpers)** | Single-rig for now |
| **Nudge channels (tmux send-keys)** | Fragile, depends on terminal state |
| **`--dangerously-skip-permissions`** | Security risk. Our capability system is better |

---

## 6. Key Source Files Reference

For future deep-dives, these are the most relevant Gastown files:

### Formulas & DAG
- `internal/formula/types.go` — Formula, Step, Leg, Var structs
- `internal/formula/parser.go` — TOML parser with type inference
- `internal/formula/dag.go` — TopologicalSort, ReadySteps, cycle detection
- `internal/formula/embed.go` — Embedded formula management
- `internal/formula/formulas/code-review.formula.toml` — Best example of convoy
- `internal/formula/formulas/mol-polecat-work.formula.toml` — Best example of workflow

### Beads (Issue Tracking)
- `internal/beads/beads.go` — Issue struct, CRUD
- `internal/beads/fields.go` — AttachmentFields, MRFields
- `internal/beads/molecule.go` — MoleculeStep, parsing

### Priming & Hooks
- `internal/cmd/prime.go` — Context injection logic
- `internal/hooks/config.go` — Hook structs, Claude Code integration
- `internal/templates/roles/` — Role-specific prompt templates

### Agent Lifecycle
- `internal/config/types.go` — TownConfig, RigConfig, RigSettings, RuntimeConfig
- `internal/config/roles/*.toml` — Role definitions (session patterns, env vars, health)

### Plugins
- `internal/plugin/types.go` — Plugin struct, Gate types
- `internal/plugin/scanner.go` — Directory scanning, plugin.md parsing

### Mail
- `internal/mail/types.go` — Message struct, routing
- `internal/mail/mailbox.go` — Beads-backed mailbox

---

## 7. Glossary

| Term | Definition |
|------|-----------|
| **Bead** | Git-backed issue/task/message. JSONL format. ID = `<prefix>-<5-char>` |
| **Convoy** | Batch of related beads tracked as a unit across rigs |
| **Formula** | Declarative workflow template (TOML). Types: convoy, workflow, expansion, aspect |
| **GUPP** | "If there is work on your Hook, YOU MUST RUN IT" — autonomous execution principle |
| **Hook** | Work attached to an agent's identity bead. Survives restarts |
| **Molecule** | Runtime instance of a formula. Persistent, tracked |
| **NDI** | "Nondeterministic Idempotence" — useful outcomes despite unreliable processes |
| **Polecat** | Worker agent with persistent identity, ephemeral sessions |
| **Rig** | A project workspace (git repo + agents + beads) |
| **Sling** | Route/assign work to an agent |
| **Town** | Top-level orchestration unit containing rigs |
| **Wisp** | Ephemeral molecule instance (auto-cleaned) |
| **ZFC** | "Zero Framework Cognition" — state from filesystem, not memory |
