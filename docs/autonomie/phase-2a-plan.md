# Phase 2a — Autonomous Execution Foundation

> Objective: Allow Ozzie to accept complex tasks, execute them in the
> background, survive crashes, and notify the user on completion.
>
> Prerequisite: Phase 1 foundations (sessions, skills, prompt composer,
> event persistence, cost tracking) — all done.

---

## Table of Contents

1. [Overview](#overview)
2. [F8: Persistent Task Queue](#f8-persistent-task-queue)
3. [F9: Heartbeat & Supervision](#f9-heartbeat--supervision)
4. [F10: Async Sub-agents](#f10-async-sub-agents)
5. [Event Bus Extensions](#event-bus-extensions)
6. [WebSocket Protocol Extensions](#websocket-protocol-extensions)
7. [Implementation Order](#implementation-order)
8. [CLI Commands](#cli-commands)
9. [Key Decisions](#key-decisions)

---

## Overview

Today Ozzie is **reactive**: the user sends a message, the agent responds,
the cycle repeats. This works for short conversations but fails for:

- Writing a complex program (hours of work across many files)
- Long-running analysis (research → plan → implement → test → iterate)
- Background maintenance (scheduled code reviews, dependency updates)
- Any task where the user disconnects mid-execution

Phase 2a adds a **proactive execution layer** that sits on top of the
existing reactive layer without modifying it.

### Target User Story

```
$ ozzie ask "Write a REST API for a library management system with tests"
→ Task created: task_abc123
→ Ozzie: "I'll work on this. I'll decompose the task and notify you when done."

# User disconnects. Ozzie keeps working.

# Later, user reconnects:
$ ozzie tasks
  task_abc123  running  "Library management REST API"  [step 3/5: writing tests]

# Eventually:
$ ozzie tasks
  task_abc123  completed  "Library management REST API"  [5/5 steps done]

$ ozzie ask --session sess_xyz "show me what you built"
→ Ozzie explains the result, references the task output.
```

### Architecture

```
                    ┌──────────────────────────────────┐
                    │         User / Client             │
                    │  (may disconnect at any time)     │
                    └──────────┬───────────────────────┘
                               │ WebSocket
                    ┌──────────▼───────────────────────┐
                    │         Gateway (Hub)             │
                    │  routes events by session         │
                    └──────────┬───────────────────────┘
                               │ Event Bus
          ┌────────────────────┼────────────────────────┐
          │                    │                        │
┌─────────▼─────────┐  ┌──────▼──────────┐  ┌──────────▼─────────┐
│   EventRunner     │  │   TaskEngine    │  │   Heartbeat        │
│   (reactive,      │  │   (proactive,   │  │   Monitor          │
│    exists)        │  │    new)         │  │   (new)            │
│                   │  │                 │  │                    │
│  user.message →   │  │  TaskStore      │  │  writes liveness   │
│  assistant.msg    │  │  WorkerPool     │  │  detects stale     │
│                   │  │  TaskRunner     │  │  triggers recovery │
└───────────────────┘  └─────────────────┘  └────────────────────┘
```

---

## F8: Persistent Task Queue

### Concept

A **Task** is a unit of work that:
- Has a lifecycle (`pending` → `running` → `completed` | `failed` | `cancelled`)
- Persists to disk (survives crashes)
- May decompose into sub-tasks
- Reports progress via events
- Produces an output artifact
- Belongs to a session (inherits session context)

### Data Model

```
~/.ozzie/tasks/
  ├── task_abc123/
  │   ├── meta.json         # Task metadata
  │   ├── plan.json         # Decomposition plan (optional)
  │   ├── checkpoints.jsonl # Checkpoint log (append-only)
  │   └── output.md         # Final output artifact
  └── task_def456/
      ├── meta.json
      └── output.md
```

**Task metadata** (`meta.json`):
```json
{
  "id": "task_abc123",
  "session_id": "sess_xyz",
  "parent_task_id": "",
  "title": "Library management REST API",
  "description": "Write a REST API for a library management system with tests",
  "status": "running",
  "priority": "normal",
  "created_at": "2026-02-23T10:00:00Z",
  "updated_at": "2026-02-23T10:15:00Z",
  "started_at": "2026-02-23T10:00:05Z",
  "completed_at": null,
  "progress": {
    "current_step": 3,
    "total_steps": 5,
    "current_step_label": "Writing unit tests",
    "percentage": 60
  },
  "plan": {
    "steps": [
      {"id": "analyze", "title": "Analyze requirements", "status": "completed"},
      {"id": "design",  "title": "Design API schema",    "status": "completed"},
      {"id": "impl",    "title": "Implement handlers",   "status": "completed"},
      {"id": "tests",   "title": "Write tests",          "status": "running"},
      {"id": "review",  "title": "Self-review & polish",  "status": "pending"}
    ]
  },
  "config": {
    "model": "anthropic/claude-sonnet-4-20250514",
    "tools": ["cmd", "read_file", "write_file", "search", "git"],
    "skill": "",
    "root_dir": "/home/user/projects/library-api"
  },
  "result": {
    "output_path": "output.md",
    "error": "",
    "token_usage": {"input": 45000, "output": 12000}
  },
  "retry_count": 0,
  "max_retries": 2
}
```

**Checkpoint** (`checkpoints.jsonl`, one JSON object per line):
```json
{"ts":"2026-02-23T10:05:00Z","step_id":"analyze","type":"step_completed","summary":"Requirements analyzed: 5 entities, 12 endpoints"}
{"ts":"2026-02-23T10:08:00Z","step_id":"design","type":"step_completed","summary":"OpenAPI schema designed with validation"}
{"ts":"2026-02-23T10:15:00Z","step_id":"impl","type":"step_completed","summary":"12 handlers implemented in internal/api/"}
{"ts":"2026-02-23T10:15:30Z","step_id":"tests","type":"step_started","summary":"Starting test suite"}
```

### Task Statuses

```
                    ┌─────────┐
              ┌────►│ pending │◄──── created, waiting for worker
              │     └────┬────┘
              │          │ worker picks up
              │     ┌────▼────┐
              │     │ running │◄──── active execution
              │     └──┬───┬──┘
              │        │   │
    retry     │   ok   │   │  error
  (if < max)  │        │   │
              │   ┌────▼┐  ▼────────┐
              │   │done ││  failed  │
              │   └─────┘└────┬─────┘
              │               │
              └───────────────┘

              ┌───────────┐
              │ cancelled │◄──── user or parent cancellation
              └───────────┘
```

### Interfaces

```go
// internal/tasks/task.go

type TaskStatus string

const (
    TaskPending   TaskStatus = "pending"
    TaskRunning   TaskStatus = "running"
    TaskCompleted TaskStatus = "completed"
    TaskFailed    TaskStatus = "failed"
    TaskCancelled TaskStatus = "cancelled"
)

type TaskPriority string

const (
    PriorityLow    TaskPriority = "low"
    PriorityNormal TaskPriority = "normal"
    PriorityHigh   TaskPriority = "high"
)

type TaskProgress struct {
    CurrentStep      int    `json:"current_step"`
    TotalSteps       int    `json:"total_steps"`
    CurrentStepLabel string `json:"current_step_label"`
    Percentage       int    `json:"percentage"`
}

type TaskPlanStep struct {
    ID     string     `json:"id"`
    Title  string     `json:"title"`
    Status TaskStatus `json:"status"`
}

type TaskConfig struct {
    Model   string   `json:"model,omitempty"`
    Tools   []string `json:"tools,omitempty"`
    Skill   string   `json:"skill,omitempty"`
    RootDir string   `json:"root_dir,omitempty"`
}

type TaskResult struct {
    OutputPath string            `json:"output_path,omitempty"`
    Error      string            `json:"error,omitempty"`
    TokenUsage sessions.TokenUsage `json:"token_usage"`
}

type Task struct {
    ID           string       `json:"id"`
    SessionID    string       `json:"session_id"`
    ParentTaskID string       `json:"parent_task_id,omitempty"`
    Title        string       `json:"title"`
    Description  string       `json:"description"`
    Status       TaskStatus   `json:"status"`
    Priority     TaskPriority `json:"priority"`
    CreatedAt    time.Time    `json:"created_at"`
    UpdatedAt    time.Time    `json:"updated_at"`
    StartedAt    *time.Time   `json:"started_at,omitempty"`
    CompletedAt  *time.Time   `json:"completed_at,omitempty"`
    Progress     TaskProgress `json:"progress"`
    Plan         *TaskPlan    `json:"plan,omitempty"`
    Config       TaskConfig   `json:"config"`
    Result       TaskResult   `json:"result"`
    RetryCount   int          `json:"retry_count"`
    MaxRetries   int          `json:"max_retries"`
}

type TaskPlan struct {
    Steps []TaskPlanStep `json:"steps"`
}
```

```go
// internal/tasks/store.go

type Store interface {
    Create(t *Task) error
    Get(id string) (*Task, error)
    List(filter ListFilter) ([]*Task, error)
    Update(t *Task) error
    Delete(id string) error
    AppendCheckpoint(taskID string, cp Checkpoint) error
    LoadCheckpoints(taskID string) ([]Checkpoint, error)
    WriteOutput(taskID string, content string) error
    ReadOutput(taskID string) (string, error)
}

type ListFilter struct {
    Status    TaskStatus   // filter by status (empty = all)
    SessionID string       // filter by session (empty = all)
    ParentID  string       // filter by parent task (empty = all)
}

type Checkpoint struct {
    Ts      time.Time `json:"ts"`
    StepID  string    `json:"step_id"`
    Type    string    `json:"type"`    // step_started, step_completed, step_failed, note
    Summary string    `json:"summary"`
}
```

**FileStore implementation**: same pattern as `sessions.FileStore` — one
directory per task, atomic writes via `.tmp` rename, `sync.RWMutex` for
thread-safety, JSONL for checkpoints.

### Worker Pool

```go
// internal/tasks/worker.go

type WorkerPool struct {
    store      Store
    bus        *events.Bus
    factory    *agent.AgentFactory
    registry   agent.ToolLookup
    models     *models.Registry
    sessStore  sessions.Store

    concurrency int           // max parallel tasks (default: 2)
    pollInterval time.Duration // how often to check for pending tasks (default: 5s)

    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

func NewWorkerPool(cfg WorkerPoolConfig) *WorkerPool

// Start begins polling for pending tasks and dispatching to workers.
func (wp *WorkerPool) Start()

// Stop gracefully shuts down all workers. Running tasks are checkpointed.
func (wp *WorkerPool) Stop()

// Submit adds a task to the store and signals the pool.
func (wp *WorkerPool) Submit(t *Task) error
```

**Worker loop** (per goroutine):
```
loop:
  1. Poll store for oldest pending task (priority-ordered)
  2. Claim task (set status = running, started_at = now)
  3. Load session context (history, root_dir, language)
  4. If task has a plan: execute steps sequentially via TaskRunner
     If no plan: ask LLM to decompose, save plan, then execute
  5. On each step completion: write checkpoint, update progress, emit event
  6. On final step: set status = completed, write output, emit event
  7. On error: increment retry_count, if < max_retries → re-queue as pending
                else set status = failed, emit event
  8. Back to loop
```

### TaskRunner

```go
// internal/tasks/runner.go

// TaskRunner executes a single task. It manages the LLM interaction loop
// for each step in the task plan.
type TaskRunner struct {
    task       *Task
    store      Store
    bus        *events.Bus
    factory    *agent.AgentFactory
    registry   agent.ToolLookup
    models     *models.Registry
    sessStore  sessions.Store
}

// Run executes the task from its current state (supports resumption).
func (tr *TaskRunner) Run(ctx context.Context) error
```

**TaskRunner execution model**:

For each plan step, the TaskRunner:
1. Creates an ephemeral agent (like `skills.WorkflowRunner.runStep`)
2. Builds an instruction that includes: task description, step instruction,
   previous step results (from checkpoints), session context
3. Runs the agent with the step's tools
4. Writes the output to a checkpoint
5. Updates task progress and emits `task.step.completed`

If no plan exists (user submitted a raw task), the first step is always
**decomposition**: an LLM call that produces a plan, which is then persisted
and executed.

### Native Tool: `submit_task`

The agent can create tasks programmatically:

```go
// internal/plugins/task_tool.go

// submit_task creates a new background task.
// Schema:
//   title:       string (required) — short task title
//   description: string (required) — detailed instructions
//   tools:       []string (optional) — tools the task can use
//   priority:    string (optional) — "low", "normal", "high"
//
// Returns: task ID
```

This enables the main agent to delegate work:
```
User: "Write a library management system"
Agent: (calls submit_task with decomposed sub-tasks)
Agent: "I've created 3 sub-tasks. I'll notify you as they complete."
```

### Package Changes

| Package | What | New/Modified |
|---------|------|-------------|
| `internal/tasks/` | `Task`, `TaskPlan`, `Checkpoint` structs | **New package** |
| `internal/tasks/` | `Store` interface + `FileStore` implementation | New |
| `internal/tasks/` | `WorkerPool` — polls and dispatches tasks | New |
| `internal/tasks/` | `TaskRunner` — executes a single task | New |
| `internal/plugins/` | `submit_task` native tool | New file |
| `internal/events/bus.go` | New event types (see [Event Bus Extensions](#event-bus-extensions)) | Modified |
| `internal/events/payloads.go` | Task-related payloads | Modified |
| `cmd/commands/gateway.go` | Wire `WorkerPool` + `submit_task` tool | Modified |
| `cmd/commands/tasks.go` | `ozzie tasks` CLI command | New |

---

## F9: Heartbeat & Supervision

### Concept

A **heartbeat** is a periodic signal proving the gateway is alive. A
**supervisor** watches the heartbeat and takes action if it stops.

### Heartbeat Writer

```go
// internal/heartbeat/heartbeat.go

type Heartbeat struct {
    PID       int       `json:"pid"`
    StartedAt time.Time `json:"started_at"`
    Timestamp time.Time `json:"timestamp"`
    Uptime    string    `json:"uptime"`
    Tasks     TaskStats `json:"tasks"`
}

type TaskStats struct {
    Pending   int `json:"pending"`
    Running   int `json:"running"`
    Completed int `json:"completed"`
    Failed    int `json:"failed"`
}

type Writer struct {
    path     string        // $OZZIE_PATH/heartbeat.json
    interval time.Duration // default: 30s
    store    tasks.Store   // to count task stats
    started  time.Time
    ctx      context.Context
    cancel   context.CancelFunc
}

func NewWriter(path string, store tasks.Store) *Writer
func (w *Writer) Start()
func (w *Writer) Stop()
```

The writer overwrites `$OZZIE_PATH/heartbeat.json` every `interval`. On
graceful shutdown, it deletes the file.

### Heartbeat Checker

```go
// internal/heartbeat/checker.go

type Status int

const (
    StatusAlive   Status = iota
    StatusStale          // heartbeat exists but too old
    StatusDead           // no heartbeat file
)

// Check reads the heartbeat file and returns the gateway status.
func Check(path string, maxAge time.Duration) (Status, *Heartbeat, error)
```

Used by:
- `ozzie status` — shows gateway health
- `ozzie ask` — warns if gateway is down
- Supervisor process (future)

### Crash Recovery

On gateway startup, before starting the worker pool:

```go
// internal/tasks/recovery.go

// RecoverTasks finds tasks that were "running" when the gateway crashed
// and resets them to "pending" so the worker pool picks them up.
func RecoverTasks(store Store) (int, error) {
    tasks, _ := store.List(ListFilter{Status: TaskRunning})
    for _, t := range tasks {
        t.Status = TaskPending
        t.UpdatedAt = time.Now()
        store.AppendCheckpoint(t.ID, Checkpoint{
            Ts:      time.Now(),
            Type:    "recovery",
            Summary: "Task recovered after gateway restart",
        })
        store.Update(t)
    }
    return len(tasks), nil
}
```

Tasks resume from their last checkpoint. The `TaskRunner` reads checkpoints
to determine which steps are already completed and skips them.

### Supervisor (launchd integration, optional)

A lightweight launchd plist that monitors the heartbeat file:

```xml
<!-- ~/Library/LaunchAgents/com.ozzie.watchdog.plist -->
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.ozzie.watchdog</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/ozzie</string>
        <string>gateway</string>
        <string>--recover</string>
    </array>
    <key>WatchPaths</key>
    <array>
        <string>~/.ozzie/heartbeat.json</string>
    </array>
    <key>KeepAlive</key>
    <dict>
        <key>PathState</key>
        <dict>
            <key>~/.ozzie/heartbeat.json</key>
            <false/>
        </dict>
    </dict>
</dict>
</plist>
```

This is optional and out of scope for the initial implementation. The
heartbeat file is the foundation — supervision can be added later with
launchd, systemd, or a custom watchdog process.

### Package Changes

| Package | What | New/Modified |
|---------|------|-------------|
| `internal/heartbeat/` | `Writer` — periodic heartbeat writer | **New package** |
| `internal/heartbeat/` | `Check` — read and evaluate heartbeat | New |
| `internal/tasks/` | `RecoverTasks` — crash recovery | New file |
| `cmd/commands/gateway.go` | Start heartbeat writer, run recovery on startup | Modified |
| `cmd/commands/status.go` | Implement `ozzie status` (reads heartbeat) | Modified |

---

## F10: Async Sub-agents

### Concept

Today, when the main agent invokes a skill (via tool call), the
`WorkflowRunner` executes **synchronously** — the main agent blocks until
the skill finishes. The WS connection stays open, streaming nothing useful.

With async sub-agents:
1. The main agent submits a **task** (via `submit_task`)
2. The worker pool picks it up in a separate goroutine
3. The main agent continues responding to the user
4. When the task completes, an event is emitted
5. The main agent is notified and can reference the result

### How It Connects to Tasks

Sub-agents are **not a separate system** — they are tasks with
`parent_task_id` set. The task engine handles them uniformly.

```
Main agent receives: "Write a library management system"
  │
  ├─ Agent decomposes into sub-tasks:
  │   ├─ submit_task("Design API schema", tools: [read_file, write_file])
  │   ├─ submit_task("Implement handlers", tools: [cmd, read_file, write_file])
  │   └─ submit_task("Write tests", tools: [cmd, read_file, write_file])
  │
  ├─ Agent responds: "I've created 3 sub-tasks. Working on them now."
  │
  └─ Worker pool processes tasks (potentially in parallel):
      ├─ task_1 (design) → completed → event: task.completed
      ├─ task_2 (implement) → running...
      └─ task_3 (tests) → pending (depends on task_2)
```

### Task Dependencies

Tasks can declare dependencies on other tasks:

```json
{
  "id": "task_impl",
  "parent_task_id": "task_main",
  "depends_on": ["task_design"],
  "title": "Implement API handlers",
  ...
}
```

The worker pool only picks up tasks whose dependencies are all `completed`.

```go
// Addition to Task struct
type Task struct {
    // ... existing fields ...
    DependsOn []string `json:"depends_on,omitempty"` // task IDs
}

// Addition to WorkerPool — task is ready when:
func (wp *WorkerPool) isReady(t *Task) bool {
    if t.Status != TaskPending {
        return false
    }
    for _, depID := range t.DependsOn {
        dep, _ := wp.store.Get(depID)
        if dep == nil || dep.Status != TaskCompleted {
            return false
        }
    }
    return true
}
```

### Notification Flow

When a task completes:

```
TaskRunner finishes step 5/5
  │
  ├─ store.Update(task) — status = completed
  ├─ bus.Publish(task.completed{task_id, session_id, output_summary})
  │
  ├─ Gateway Hub receives event → sends to connected WS clients
  │   └─ Client sees: "Task 'Write tests' completed successfully"
  │
  └─ If parent_task_id is set:
      └─ Check if all sibling tasks are done
          └─ If yes: mark parent as completed, emit task.completed for parent
```

If the user is **not connected**, the notification is stored in the session
as a system message. When the user reconnects and loads history, they see:

```json
{"role":"system","content":"[Task completed] 'Library management REST API' finished successfully. Use `ozzie tasks show task_abc123` to see details.","ts":"2026-02-23T11:30:00Z"}
```

### Cancellation

```go
// internal/tasks/worker.go

// Cancel stops a running task. If the task has sub-tasks, they are
// cancelled recursively.
func (wp *WorkerPool) Cancel(taskID string) error {
    // 1. Cancel the context for the running TaskRunner
    // 2. Set status = cancelled
    // 3. Cancel child tasks (where parent_task_id == taskID)
    // 4. Emit task.cancelled event
}
```

Each `TaskRunner` runs in a context derived from the worker pool. The pool
maintains a `map[string]context.CancelFunc` for active runners. Cancellation
is propagated via context.

### Native Tool: `check_task`

The main agent can query task status:

```go
// internal/plugins/task_tool.go

// check_task returns the current status of a task.
// Schema:
//   task_id: string (required) — the task to check
//
// Returns: JSON with status, progress, output summary
```

This lets the agent reason about task progress:
```
User: "How's the library API going?"
Agent: (calls check_task("task_abc123"))
Agent: "It's 60% done — currently writing tests. Steps 1-3 are complete."
```

### Package Changes

| Package | What | New/Modified |
|---------|------|-------------|
| `internal/tasks/` | `DependsOn` field on Task, dependency resolution in WorkerPool | Modified |
| `internal/tasks/` | `Cancel` method on WorkerPool | Modified |
| `internal/plugins/` | `check_task` native tool | New file |
| `internal/agent/eventrunner.go` | Subscribe to `task.completed` → inject system message in session | Modified |
| `internal/gateway/ws/hub.go` | Forward task events to WS clients | Modified (event routing already works) |

---

## Event Bus Extensions

### New Event Types

```go
// internal/events/bus.go — additions

const (
    // Task lifecycle
    EventTaskCreated   EventType = "task.created"
    EventTaskStarted   EventType = "task.started"
    EventTaskProgress  EventType = "task.progress"
    EventTaskCompleted EventType = "task.completed"
    EventTaskFailed    EventType = "task.failed"
    EventTaskCancelled EventType = "task.cancelled"

    // Heartbeat
    EventHeartbeat EventType = "heartbeat"
)
```

### New Payloads

```go
// internal/events/payloads.go — additions

type TaskCreatedPayload struct {
    TaskID      string `json:"task_id"`
    Title       string `json:"title"`
    Description string `json:"description"`
    ParentID    string `json:"parent_id,omitempty"`
}
func (TaskCreatedPayload) EventType() EventType { return EventTaskCreated }

type TaskStartedPayload struct {
    TaskID string `json:"task_id"`
    Title  string `json:"title"`
}
func (TaskStartedPayload) EventType() EventType { return EventTaskStarted }

type TaskProgressPayload struct {
    TaskID           string `json:"task_id"`
    CurrentStep      int    `json:"current_step"`
    TotalSteps       int    `json:"total_steps"`
    CurrentStepLabel string `json:"current_step_label"`
    Percentage       int    `json:"percentage"`
}
func (TaskProgressPayload) EventType() EventType { return EventTaskProgress }

type TaskCompletedPayload struct {
    TaskID       string        `json:"task_id"`
    Title        string        `json:"title"`
    OutputPath   string        `json:"output_path,omitempty"`
    Duration     time.Duration `json:"duration"`
    TokenUsage   TokenUsage    `json:"token_usage"`
}
func (TaskCompletedPayload) EventType() EventType { return EventTaskCompleted }

type TaskFailedPayload struct {
    TaskID     string `json:"task_id"`
    Title      string `json:"title"`
    Error      string `json:"error"`
    RetryCount int    `json:"retry_count"`
    WillRetry  bool   `json:"will_retry"`
}
func (TaskFailedPayload) EventType() EventType { return EventTaskFailed }

type TaskCancelledPayload struct {
    TaskID string `json:"task_id"`
    Reason string `json:"reason"`
}
func (TaskCancelledPayload) EventType() EventType { return EventTaskCancelled }

type HeartbeatPayload struct {
    PID    int       `json:"pid"`
    Uptime string    `json:"uptime"`
    Tasks  TaskStats `json:"tasks"`
}
func (HeartbeatPayload) EventType() EventType { return EventHeartbeat }
```

---

## WebSocket Protocol Extensions

### New Methods (client → server)

```json
// Submit a task
{"type":"req","id":5,"method":"submit_task","params":{
  "title":"Write REST API",
  "description":"Full library management system with tests",
  "priority":"normal"
}}
// Response:
{"type":"res","id":5,"result":{"task_id":"task_abc123"}}

// Check task status
{"type":"req","id":6,"method":"check_task","params":{"task_id":"task_abc123"}}
// Response:
{"type":"res","id":6,"result":{
  "id":"task_abc123","status":"running",
  "progress":{"current_step":3,"total_steps":5,"percentage":60}
}}

// Cancel a task
{"type":"req","id":7,"method":"cancel_task","params":{"task_id":"task_abc123"}}
// Response:
{"type":"res","id":7,"result":{"cancelled":true}}

// List tasks
{"type":"req","id":8,"method":"list_tasks","params":{"status":"running"}}
// Response:
{"type":"res","id":8,"result":{"tasks":[...]}}
```

### New Events (server → client)

Task events are forwarded to connected clients via the existing event
bridging in the Hub. No special handling needed — the Hub already subscribes
to all events and routes by session ID.

```json
// Task progress (periodic)
{"type":"event","event":"task.progress","data":{
  "task_id":"task_abc123","current_step":3,"total_steps":5,
  "current_step_label":"Writing tests","percentage":60
}}

// Task completed
{"type":"event","event":"task.completed","data":{
  "task_id":"task_abc123","title":"Library REST API",
  "duration":"25m30s"
}}
```

---

## Implementation Order

Foundations are ordered by dependency:

```
F9: Heartbeat         ──── FIRST (simplest, no dependencies)
  │
  v
F8: Task Queue        ──── SECOND (core of the system)
  │    │
  │    ├── Task struct + Store + FileStore
  │    ├── WorkerPool + TaskRunner
  │    ├── Crash recovery (uses heartbeat)
  │    └── submit_task tool
  │
  v
F10: Async Sub-agents ──── THIRD (builds on task queue)
       │
       ├── Task dependencies
       ├── check_task tool
       ├── Cancellation
       └── Notification injection in sessions
```

### Estimated Scope

| Foundation | New files | Modified files | New package? | Complexity |
|------------|-----------|----------------|-------------|------------|
| F9: Heartbeat | 2-3 | 2 | `internal/heartbeat/` | Low |
| F8: Task Queue | 5-7 | 4 | `internal/tasks/` | High |
| F10: Async Sub-agents | 2-3 | 3 | (extends `internal/tasks/`) | Medium |

### Milestones

**M1: Heartbeat** (F9)
- [ ] `internal/heartbeat/heartbeat.go` — Writer + Checker
- [ ] `cmd/commands/gateway.go` — start writer on gateway boot
- [ ] `cmd/commands/status.go` — implement `ozzie status`
- [ ] Tests for heartbeat writer and checker

**M2: Task Store** (F8, part 1)
- [ ] `internal/tasks/task.go` — Task, Checkpoint, Status types
- [ ] `internal/tasks/store.go` — Store interface
- [ ] `internal/tasks/filestore.go` — FileStore implementation
- [ ] Tests for FileStore

**M3: Task Execution** (F8, part 2)
- [ ] `internal/tasks/runner.go` — TaskRunner (single task execution)
- [ ] `internal/tasks/worker.go` — WorkerPool (polling + dispatch)
- [ ] `internal/tasks/recovery.go` — crash recovery
- [ ] `internal/events/` — task event types + payloads
- [ ] `internal/plugins/task_tool.go` — `submit_task` native tool
- [ ] `cmd/commands/gateway.go` — wire worker pool
- [ ] `cmd/commands/tasks.go` — `ozzie tasks list/show/cancel`
- [ ] Tests for worker pool and runner

**M4: Async Sub-agents** (F10)
- [ ] Task dependencies (`depends_on` field + resolution)
- [ ] `internal/plugins/task_tool.go` — `check_task` native tool
- [ ] Cancellation propagation
- [ ] `internal/agent/eventrunner.go` — notification injection
- [ ] `internal/gateway/ws/hub.go` — WS protocol methods
- [ ] Tests for dependencies and cancellation

---

## CLI Commands

### `ozzie tasks`

```
$ ozzie tasks
ID            STATUS     TITLE                          PROGRESS
task_abc123   running    Library management REST API     [████░░░░░░] 60%
task_def456   pending    Update CI pipeline             waiting

$ ozzie tasks show task_abc123
Task: task_abc123
Title: Library management REST API
Status: running (started 25m ago)
Session: sess_xyz
Progress: step 3/5 — Writing unit tests

Steps:
  ✓ analyze    Analyze requirements
  ✓ design     Design API schema
  ✓ impl       Implement handlers
  ▸ tests      Write tests
  ○ review     Self-review & polish

$ ozzie tasks cancel task_abc123
Task task_abc123 cancelled.
```

### `ozzie status` (enhanced)

```
$ ozzie status
Gateway: alive (PID 12345, uptime 2h15m)
Tasks:   1 running, 2 pending, 5 completed, 0 failed
Sessions: 3 active
Model: anthropic/claude-sonnet-4-20250514
```

---

## Key Decisions

### Task Queue vs Message Queue

**Decision: File-based task store, not a message queue.**

Rationale: Ozzie is a personal agent, not a distributed system. We need
persistence and crash recovery, not high-throughput message passing. A file
per task (like sessions) is simple, inspectable, and Git-friendly. If scale
becomes a concern later, the `Store` interface allows swapping in SQLite or
Redis.

### Worker Pool vs goroutine-per-task

**Decision: Fixed-size worker pool (default: 2 concurrent tasks).**

Rationale: LLM calls are expensive and rate-limited. Running 10 tasks in
parallel would burn tokens and hit API limits. 2 workers is a reasonable
default for a personal agent — one for the current high-priority task, one
for background work.

### Sub-agents as Tasks vs Separate System

**Decision: Sub-agents are tasks with `parent_task_id`.**

Rationale: No reason to have two parallel execution systems. A "sub-agent"
is just a task that was created by another task. The worker pool handles
both uniformly. Dependencies between tasks handle ordering. This is simpler
and more composable.

### Heartbeat File vs TCP Health Check

**Decision: File-based heartbeat (`heartbeat.json`).**

Rationale: A file is observable without a running process (launchd can watch
it). TCP health checks require a client to poll. The heartbeat file also
carries useful metadata (task stats, uptime) that a TCP ping wouldn't.

### Task Decomposition: Agent-driven vs User-driven

**Decision: Agent decomposes tasks automatically, user can override.**

When the agent receives a complex task via `submit_task`:
1. If no `plan` is provided: the first step is always "decompose this task
   into concrete steps"
2. The LLM produces a plan, which is persisted
3. The plan is then executed step by step

The user can also provide an explicit plan by calling `submit_task` with
pre-defined steps, or by using a skill (which already has a DAG).

### Relationship to Skills

Skills and tasks are complementary:
- **Skills** define reusable workflows (JSONC DAGs)
- **Tasks** are runtime instances of work (may use a skill or not)

A task can reference a skill in its config:
```json
{"config": {"skill": "code-review"}}
```

When set, the TaskRunner delegates to the skill's WorkflowRunner instead
of running its own LLM decomposition loop. The skill provides the structure,
the task provides the instance state.

---

## Future Extensions (out of scope for 2a)

These build naturally on the Phase 2a foundation:

- **Task history & analytics** — query completed tasks, token costs per task
- **Task templates** — pre-defined task structures (like skills but more operational)
- **Remote notifications** — webhook, Slack, email on task completion
- **Watchdog process** — launchd/systemd supervisor using heartbeat
- **Task priorities & preemption** — pause low-priority tasks when high-priority arrives
- **Distributed workers** — run workers on remote machines (uses the Store interface)
