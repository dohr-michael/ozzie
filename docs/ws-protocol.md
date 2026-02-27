# Ozzie WebSocket Protocol v1

> Reference specification for building Ozzie connectors (TUI, Web, Discord, Slack, ...).
> All connectors communicate with the Ozzie Gateway through this single protocol.

## Table of Contents

- [Overview](#overview)
- [Connection](#connection)
- [Frame Format](#frame-format)
- [Methods (Client → Server)](#methods-client--server)
- [Events (Server → Client)](#events-server--client)
- [Flows](#flows)
- [HTTP Endpoints](#http-endpoints)
- [Implementing a Connector](#implementing-a-connector)

---

## Overview

```
┌──────────┐         WebSocket (JSON frames)         ┌──────────────┐
│ Connector│◄───────────────────────────────────────►│ Ozzie Gateway│
│ (TUI,Web)│  req/res (RPC) + events (server-push)   │  :7777       │
└──────────┘                                          └──────┬───────┘
                                                             │
                                                       ┌─────▼─────┐
                                                       │ Event Bus │
                                                       └─────┬─────┘
                                                             │
                                                ┌────────────┼────────────┐
                                                │            │            │
                                          ┌─────▼──┐  ┌─────▼──┐  ┌─────▼──┐
                                          │ Agent  │  │ Tasks  │  │ Skills │
                                          └────────┘  └────────┘  └────────┘
```

The protocol has two directions:

| Direction | Frame Type | Purpose |
|-----------|-----------|---------|
| Client → Server | `req` | RPC calls (open session, send message, ...) |
| Server → Client | `res` | Response to an RPC call (matched by `id`) |
| Server → Client | `event` | Real-time push (streaming tokens, tool calls, ...) |

Transport: **WebSocket text frames**, each containing one JSON object.

---

## Connection

### Endpoint

```
ws://localhost:7777/api/ws
```

### Lifecycle

```
1. Connect    → WebSocket handshake to /api/ws
2. Open       → send "open_session" request
3. Interact   → send messages, receive events
4. Disconnect → close WebSocket (server cleans up)
```

- The server accepts multiple concurrent connections.
- Multiple clients can join the same session (collaborative).
- When the **last** client in a session disconnects, the session is closed.

### Keep-alive

No explicit ping/pong needed — the `coder/websocket` library handles this automatically.

---

## Frame Format

Every WebSocket message is a JSON object with this envelope:

```typescript
interface Frame {
  type: "req" | "res" | "event";

  // Request fields
  id?:     string;          // correlation ID (client-generated, echoed in response)
  method?: string;          // RPC method name
  params?: object;          // method parameters

  // Response fields
  ok?:      boolean;        // success flag
  payload?: object;         // response data (on success)
  error?:   string;         // error message (on failure)

  // Event fields
  event?:      string;      // event type name
  session_id?: string;      // session scope
}
```

### Request (`type: "req"`)

```json
{
  "type": "req",
  "id": "req-1",
  "method": "send_message",
  "params": { "content": "Hello Ozzie" }
}
```

The `id` must be unique per connection. Recommended format: `req-{incrementing_counter}`.

### Response (`type: "res"`)

Success:
```json
{
  "type": "res",
  "id": "req-1",
  "ok": true,
  "payload": { "status": "sent" }
}
```

Error:
```json
{
  "type": "res",
  "id": "req-1",
  "ok": false,
  "error": "session not found: sess_xyz"
}
```

### Event (`type: "event"`)

```json
{
  "type": "event",
  "event": "assistant.stream",
  "session_id": "sess_abc123",
  "payload": {
    "phase": "delta",
    "content": "Hello! ",
    "index": 0
  }
}
```

Events are pushed by the server without a prior request. They are scoped by `session_id` — a client only receives events for sessions it has joined.

---

## Methods (Client → Server)

### `open_session`

Create a new session or resume an existing one.

**Params:**
```json
{
  "session_id": "",
  "root_dir": "/home/user/project"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | no | Empty = create new, non-empty = resume existing |
| `root_dir` | string | no | Working directory for tools (default: gateway cwd) |

**Response payload:**
```json
{
  "session_id": "sess_abc123",
  "status": "created"
}
```

`status` is `"created"` or `"resumed"`.

---

### `send_message`

Send a user message to the agent. This triggers the LLM inference loop.

**Params:**
```json
{
  "content": "What files are in the current directory?"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `content` | string | yes | The user's message text |

**Response payload:**
```json
{ "status": "sent" }
```

> **Note:** If no session has been opened, the server auto-creates one. The response is immediate — the actual LLM output arrives as `assistant.stream` and `assistant.message` events.

---

### `prompt_response`

Respond to an interactive prompt (tool confirmation, password input, etc.).

**Params:**
```json
{
  "token": "prompt_abc123",
  "value": "yes",
  "cancelled": false
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `token` | string | yes | Token from the `prompt.request` event |
| `value` | any | no | User's answer (string, bool, string[], depending on prompt type) |
| `cancelled` | bool | no | `true` to cancel/reject the prompt |

**Response payload:**
```json
{ "status": "sent" }
```

---

### `accept_all_tools`

Auto-approve all tool calls for this session (skip confirmation prompts).

**Params:** _(none)_

**Response payload:**
```json
{ "status": "accepted" }
```

---

### `submit_task`

Submit an asynchronous task for background execution.

**Params:**
```json
{
  "title": "Refactor auth module",
  "description": "Split the auth module into separate concerns...",
  "tools": ["run_command", "git"],
  "priority": "normal"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | string | yes | Short task title |
| `description` | string | yes | Detailed task description |
| `tools` | string[] | no | Allowed tools (default: `["run_command", "git", "query_memories"]`) |
| `priority` | string | no | Priority level |

**Response payload:**
```json
{
  "task_id": "task_xyz",
  "status": "submitted"
}
```

---

### `check_task`

Get the current status of a task.

**Params:**
```json
{ "task_id": "task_xyz" }
```

**Response payload:**
```json
{
  "id": "task_xyz",
  "title": "Refactor auth module",
  "status": "running",
  "progress": {
    "current_step": 2,
    "total_steps": 5,
    "percentage": 40
  }
}
```

---

### `cancel_task`

Cancel a running or pending task.

**Params:**
```json
{
  "task_id": "task_xyz",
  "reason": "No longer needed"
}
```

**Response payload:**
```json
{
  "task_id": "task_xyz",
  "status": "cancelled"
}
```

---

### `list_tasks`

List all tasks for the current session.

**Params:** _(none — uses session from connection)_

**Response payload:** Array of task summaries.

---

### `reply_task`

Send feedback to a suspended task (the agent paused waiting for user input).

**Params:**
```json
{
  "task_id": "task_xyz",
  "feedback": "Looks good, proceed with the refactor",
  "status": "approved"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `task_id` | string | yes | The suspended task ID |
| `feedback` | string | no | User feedback text |
| `status` | string | yes | `"approved"` or `"revise"` |

**Response payload:**
```json
{
  "task_id": "task_xyz",
  "status": "replied"
}
```

---

## Events (Server → Client)

Events are pushed in real-time. The `payload` field contains event-specific data.

### Assistant Events

#### `assistant.stream`

Streaming LLM output — the main way to display text as it's generated.

```json
{
  "event": "assistant.stream",
  "payload": {
    "phase": "start" | "delta" | "end",
    "content": "partial text...",
    "index": 0
  }
}
```

| Phase | Meaning |
|-------|---------|
| `start` | A new stream begins. Clear/prepare the output area. |
| `delta` | A chunk of text. Append to current output. |
| `end` | Stream finished. Finalize the output. |

`index` identifies the stream when multiple streams run in parallel (usually `0`).

#### `assistant.message`

Final complete message from the assistant (sent after stream ends).

```json
{
  "event": "assistant.message",
  "payload": {
    "content": "The full response text...",
    "error": "",
    "context": {}
  }
}
```

If `error` is non-empty, the LLM call failed.

---

### Tool Events

#### `tool.call`

A tool invocation lifecycle.

```json
{
  "event": "tool.call",
  "payload": {
    "status": "started" | "completed" | "failed",
    "name": "run_command",
    "arguments": { "cmd": "ls -la" },
    "result": "...",
    "error": ""
  }
}
```

| Status | Fields present | Meaning |
|--------|---------------|---------|
| `started` | `name`, `arguments` | Tool execution begins |
| `completed` | `name`, `result` | Tool succeeded |
| `failed` | `name`, `error` | Tool errored |

**Connector guidance:** Show a spinner/indicator on `started`, display result on `completed`, show error on `failed`.

---

### Prompt Events

#### `prompt.request`

The server needs user input. The connector **must** display a UI and reply with `prompt_response`.

```json
{
  "event": "prompt.request",
  "payload": {
    "type": "confirm",
    "label": "Allow execution of 'rm -rf /tmp/build'?",
    "token": "prompt_abc123",
    "required": true,
    "options": [],
    "default": null,
    "placeholder": "",
    "validation": "",
    "context": {}
  }
}
```

**Prompt types:**

| Type | Expected `value` | UI Element |
|------|------------------|------------|
| `text` | `string` | Text input field |
| `select` | `string` | Dropdown / radio buttons (single choice from `options`) |
| `multi` | `string[]` | Checkboxes (multiple choices from `options`) |
| `confirm` | `bool` | Yes/No button |
| `password` | `string` | Masked input (value is encrypted in transit) |

**Option format** (for `select` and `multi`):
```json
{
  "value": "opt1",
  "label": "Option One",
  "description": "Longer description",
  "disabled": false
}
```

> **Important:** If the user dismisses/cancels the prompt, send `{"token": "...", "cancelled": true}`.

---

### Session Events

#### `session.created`
```json
{ "event": "session.created", "payload": {} }
```

#### `session.closed`
```json
{ "event": "session.closed", "payload": {} }
```

---

### Task Events

All task events include `task_id` in the payload.

#### `task.created`
```json
{
  "event": "task.created",
  "payload": {
    "task_id": "task_xyz",
    "title": "Refactor auth",
    "description": "...",
    "parent_id": ""
  }
}
```

#### `task.started`
```json
{
  "event": "task.started",
  "payload": { "task_id": "task_xyz", "title": "Refactor auth" }
}
```

#### `task.progress`
```json
{
  "event": "task.progress",
  "payload": {
    "task_id": "task_xyz",
    "current_step": 2,
    "total_steps": 5,
    "current_step_label": "Running tests",
    "percentage": 40
  }
}
```

#### `task.completed`
```json
{
  "event": "task.completed",
  "payload": {
    "task_id": "task_xyz",
    "title": "Refactor auth",
    "output_summary": "Refactored into 3 modules...",
    "duration": 45000000000,
    "tokens_input": 1200,
    "tokens_output": 3400
  }
}
```

#### `task.failed`
```json
{
  "event": "task.failed",
  "payload": {
    "task_id": "task_xyz",
    "title": "Refactor auth",
    "error": "compilation failed",
    "retry_count": 1,
    "will_retry": true
  }
}
```

#### `task.cancelled`
```json
{
  "event": "task.cancelled",
  "payload": { "task_id": "task_xyz", "reason": "User requested" }
}
```

#### `task.suspended`

The task is paused and waiting for user feedback. Reply with `reply_task` method.

```json
{
  "event": "task.suspended",
  "payload": {
    "task_id": "task_xyz",
    "title": "Refactor auth",
    "reason": "Need approval for database schema change",
    "suspend_count": 1,
    "plan_content": "I propose to add a new `roles` table...",
    "token": "suspend_token_123"
  }
}
```

#### `task.resumed`
```json
{
  "event": "task.resumed",
  "payload": { "task_id": "task_xyz", "title": "Refactor auth" }
}
```

---

### Skill Events

#### `skill.started`
```json
{
  "event": "skill.started",
  "payload": { "skill_name": "deploy", "type": "multi-step", "vars": {} }
}
```

#### `skill.completed`
```json
{
  "event": "skill.completed",
  "payload": {
    "skill_name": "deploy",
    "output": "Deployed to production",
    "error": "",
    "duration": 12000000000
  }
}
```

#### `skill.step.started` / `skill.step.completed`
```json
{
  "event": "skill.step.started",
  "payload": {
    "skill_name": "deploy",
    "step_id": "build",
    "step_title": "Building project",
    "model": "claude-sonnet-4-6"
  }
}
```

---

### Internal Events

#### `internal.llm.call`

LLM call telemetry (useful for cost tracking / debugging).

```json
{
  "event": "internal.llm.call",
  "payload": {
    "phase": "start" | "end",
    "model": "claude-sonnet-4-6",
    "provider": "anthropic",
    "message_count": 5,
    "tokens_input": 1200,
    "tokens_output": 450,
    "duration": 3200000000,
    "error": ""
  }
}
```

#### `schedule.trigger` / `schedule.created` / `schedule.removed`

Scheduler lifecycle events.

---

## Flows

### Basic Conversation

```
Client                          Server
  │                               │
  ├─req: open_session ──────────►│
  │◄──────────── res: session_id──┤
  │                               │
  ├─req: send_message ──────────►│
  │◄──────────────────── res: ok──┤
  │                               │
  │◄─event: assistant.stream ─────┤  (phase: start)
  │◄─event: assistant.stream ─────┤  (phase: delta, content: "Hello")
  │◄─event: assistant.stream ─────┤  (phase: delta, content: " world!")
  │◄─event: assistant.stream ─────┤  (phase: end)
  │◄─event: assistant.message ────┤  (full content)
  │                               │
```

### Conversation with Tool Call

```
Client                          Server
  │                               │
  ├─req: send_message ──────────►│  "List the files"
  │◄──────────────────── res: ok──┤
  │                               │
  │◄─event: tool.call ────────────┤  (status: started, name: run_command)
  │◄─event: prompt.request ───────┤  (type: confirm, "Allow run_command?")
  │                               │
  ├─req: prompt_response ───────►│  (token: ..., cancelled: false)
  │◄──────────────────── res: ok──┤
  │                               │
  │◄─event: tool.call ────────────┤  (status: completed, result: "file1\nfile2")
  │                               │
  │◄─event: assistant.stream ─────┤  (streaming response about files)
  │◄─event: assistant.message ────┤  (final)
  │                               │
```

### Async Task

```
Client                          Server
  │                               │
  ├─req: submit_task ───────────►│  "Refactor auth module"
  │◄───── res: task_id, status────┤
  │                               │
  │◄─event: task.created ─────────┤
  │◄─event: task.started ─────────┤
  │◄─event: task.progress ────────┤  (step 1/3)
  │◄─event: task.progress ────────┤  (step 2/3)
  │◄─event: task.suspended ───────┤  "Need approval for schema change"
  │                               │
  ├─req: reply_task ────────────►│  (status: approved)
  │◄──────────────────── res: ok──┤
  │                               │
  │◄─event: task.resumed ─────────┤
  │◄─event: task.progress ────────┤  (step 3/3)
  │◄─event: task.completed ───────┤
  │                               │
```

---

## HTTP Endpoints

These REST endpoints are available for non-WebSocket access (debugging, monitoring).

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Health check → `{"status":"ok"}` |
| `GET` | `/api/ws` | WebSocket upgrade endpoint |
| `GET` | `/api/events?limit=50` | Recent event history (ring buffer) |
| `GET` | `/api/sessions` | List all sessions |
| `GET` | `/api/tasks?session_id=...` | List tasks (optional session filter) |

---

## Implementing a Connector

### Minimum Viable Connector

A basic connector needs to handle just 4 things:

1. **Connect** and call `open_session`
2. **Send** user input via `send_message`
3. **Render** `assistant.stream` events (append `delta` content to display)
4. **Handle** `prompt.request` events (at minimum: confirm prompts)

### Event Handling Matrix

Which events a connector should handle, by complexity tier:

| Event | Tier 1 (Basic) | Tier 2 (Standard) | Tier 3 (Full) |
|-------|:-:|:-:|:-:|
| `assistant.stream` | **Required** | **Required** | **Required** |
| `assistant.message` | **Required** | **Required** | **Required** |
| `prompt.request` (confirm) | **Required** | **Required** | **Required** |
| `prompt.request` (text/select) | — | **Required** | **Required** |
| `prompt.request` (password) | — | Recommended | **Required** |
| `tool.call` | — | **Required** | **Required** |
| `task.*` | — | — | **Required** |
| `skill.*` | — | — | **Required** |
| `internal.llm.call` | — | — | Optional |
| `schedule.*` | — | — | Optional |

### Pseudocode Reference

```python
ws = connect("ws://localhost:7777/api/ws")
req_id = 0

# 1. Open session
req_id += 1
ws.send({"type": "req", "id": f"req-{req_id}", "method": "open_session", "params": {"root_dir": cwd}})
res = ws.recv()  # type: "res", ok: true
session_id = res["payload"]["session_id"]

# 2. Send message
req_id += 1
ws.send({"type": "req", "id": f"req-{req_id}", "method": "send_message", "params": {"content": user_input}})

# 3. Event loop
while True:
    frame = ws.recv()

    if frame["type"] == "res":
        # Handle RPC response (usually just ack)
        continue

    if frame["type"] == "event":
        match frame["event"]:
            case "assistant.stream":
                p = frame["payload"]
                if p["phase"] == "delta":
                    display_append(p["content"])
                elif p["phase"] == "end":
                    display_newline()

            case "assistant.message":
                # Final message — can be used for non-streaming fallback
                pass

            case "prompt.request":
                p = frame["payload"]
                answer = show_prompt(p["type"], p["label"], p["options"])
                req_id += 1
                ws.send({
                    "type": "req",
                    "id": f"req-{req_id}",
                    "method": "prompt_response",
                    "params": {"token": p["token"], "value": answer}
                })

            case "tool.call":
                p = frame["payload"]
                if p["status"] == "started":
                    show_spinner(f"Running {p['name']}...")
                elif p["status"] == "completed":
                    hide_spinner()
                    show_tool_result(p["result"])
                elif p["status"] == "failed":
                    hide_spinner()
                    show_error(p["error"])
```

### Notes for Specific Platforms

**Discord / Slack:**
- Streaming (`assistant.stream`) may not be practical — use `assistant.message` for the final result
- Map `prompt.request` (confirm) to reaction buttons or interactive components
- Consider `accept_all_tools` to skip confirmation prompts in trusted contexts

**Web UI:**
- Use `assistant.stream` deltas for real-time typing effect
- Render tool calls as expandable panels
- Task events map naturally to a sidebar/panel with progress bars

**TUI:**
- Stream deltas directly to terminal
- Use inline prompts for `prompt.request`
- Tool calls can show as indented, styled lines
