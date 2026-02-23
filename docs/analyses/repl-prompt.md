# Ozzie TUI — REPL Specification

**Context:** REPL/TUI inspired by Claude Code, built in Go. The application is strictly **Event Sourced**. The TUI is a pure consumer of events from the gateway via WebSocket and projects visual state in real-time. It has no business state of its own.

## Stack

| What | Technology |
|------|-----------|
| Language | Go 1.25 |
| UI Framework | `charmbracelet/bubbletea`, `bubbles`, `lipgloss` |
| Markdown rendering | `charmbracelet/glamour` |
| Architecture | Atomic Design (Atoms, Molecules, Organisms) |
| Source of truth | Event stream via WebSocket (`ws://localhost:18420/api/ws`) |
| WS client | `clients/ws` (already implemented) |

## Event Contract

The TUI must handle these events from the gateway (see `internal/events/payloads.go`):

### Streaming (assistant text)

| Event | Payload | TUI action |
|-------|---------|------------|
| `assistant.stream` phase=`start` | `AssistantStreamPayload{Phase: "start"}` | Append new `TextBlock` to history, show spinner |
| `assistant.stream` phase=`delta` | `AssistantStreamPayload{Phase: "delta", Content, Index}` | Append `Content` to current `TextBlock` in-place |
| `assistant.stream` phase=`end` | `AssistantStreamPayload{Phase: "end"}` | Stop spinner, finalize block |
| `assistant.message` | `AssistantMessagePayload{Content, Error}` | Final message (or error display) |

### Tool calls

| Event | Payload | TUI action |
|-------|---------|------------|
| `tool.call` status=`started` | `ToolCallPayload{Status: "started", Name, Arguments}` | Append new collapsed `ToolBlock`, show spinner |
| `tool.call` status=`completed` | `ToolCallPayload{Status: "completed", Name, Result}` | Stop spinner, show checkmark, store result |
| `tool.call` status=`failed` | `ToolCallPayload{Status: "failed", Name, Error}` | Stop spinner, show error icon, store error |

### Prompts (agent → user interaction)

| Event | Payload | TUI action |
|-------|---------|------------|
| `prompt.request` | `PromptRequestPayload{Type, Label, Options, Token, ...}` | Switch to `FormHandler` mode (Select/MultiSelect/Confirmation/Text) |
| → response sent via | `MethodPromptResponse` frame with `PromptResponsePayload{Value, Token}` | Return to normal input mode |

### Skills

| Event | Payload | TUI action |
|-------|---------|------------|
| `skill.started` | `SkillStartedPayload{SkillName, Type}` | Show skill activity indicator |
| `skill.step.started` | `SkillStepStartedPayload{SkillName, StepName}` | Update step label |
| `skill.step.completed` | `SkillStepCompletedPayload{SkillName, StepName}` | Mark step done |
| `skill.completed` | `SkillCompletedPayload{SkillName, Output, Error, Duration}` | Finalize skill block |

### Session lifecycle

| Event | TUI action |
|-------|------------|
| `session.created` | Store session ID, show welcome |
| `session.closed` | Show disconnection notice |

---

## Instructions

### 1. Bootstrap (`ozzie tui` command)

The `cmd/commands/tui.go` command must:

1. Load config via `config.Load()`
2. Dial gateway via `ws.Dial(ctx, url)` using configured host/port
3. Open session via `client.OpenSession(opts)` (new or resume)
4. Start a background goroutine that calls `client.ReadFrame()` in a loop and converts frames to `tea.Msg` via `p.Send()`
5. Launch `tea.NewProgram(NewMainModel(client))` with `tea.WithAltScreen()`
6. On exit: `client.Close()`

### 2. Atomic Architecture (folder structure)

```
clients/tui/
├── doc.go              ← package doc (exists)
├── main.go             ← MainModel, tea.Model implementation
├── project.go          ← Project(event) → tea.Msg conversion
├── theme.go            ← colors, styles, lipgloss definitions
├── atoms/
│   ├── spinner.go      ← animated spinner (wrap bubbles/spinner)
│   ├── label.go        ← StyledLabel (role indicator, status badges)
│   └── caret.go        ← blinking cursor for streaming
├── molecules/
│   ├── input.go        ← CommandInput: wrap bubbles/textarea
│   │                      Alt+Enter = newline, Enter = submit
│   ├── toolheader.go   ← ToolHeader: icon + name + status + duration
│   └── suggestion.go   ← SuggestionOverlay: slash command autocomplete
└── organisms/
    ├── viewport.go     ← OutputViewport: wrap bubbles/viewport
    │                      contains []ContentBlock (interface)
    ├── toolblock.go    ← ToolBlock: collapsible tool call display
    ├── textblock.go    ← TextBlock: streaming markdown text
    ├── skillblock.go   ← SkillBlock: skill execution with steps
    └── form.go         ← FormHandler: dynamic prompt UI
```

### 3. Content Block Interface

```go
// ContentBlock represents a renderable block in the output viewport.
type ContentBlock interface {
    tea.Model
    // BlockID returns a unique identifier for this block.
    BlockID() string
    // IsComplete returns true when the block is finalized.
    IsComplete() bool
}
```

Implementations: `TextBlock`, `ToolBlock`, `SkillBlock`.

### 4. Event Projection (the "Projector")

The TUI has no business state. It implements projection via `Project()`:

```go
// Project converts a WS Frame (type="event") into a tea.Msg.
// Called by the background reader goroutine.
func Project(frame ws.Frame) tea.Msg
```

Rules:
- Each event appends to or mutates the visual history (slice of `ContentBlock`)
- **Streaming optimization:** `assistant.stream` phase=`delta` must update the last `TextBlock` in-place — no full viewport redraw. Use `Index` for ordering.
- **Tool blocks:** Created on `tool.call` status=`started`, updated on `completed`/`failed`
- **Prompt mode:** `prompt.request` activates `FormHandler`, hides text input. On response, deactivates form and restores input.

### 5. Input Behavior (sticky bottom)

- Input is anchored to the bottom via `lipgloss.JoinVertical(lipgloss.Top, viewport, input)`
- Layout: `viewport` takes all available height minus input height
- When `FormHandler` is active, text input is hidden and replaced by the form component
- **Slash commands:** Typing `/` triggers `SuggestionOverlay` with autocomplete (`/clear`, `/model`, `/session`, `/quit`)

### 6. Keybindings

| Key | Action |
|-----|--------|
| `Enter` | Submit message (single-line mode) |
| `Alt+Enter` | Insert newline (multi-line editing) |
| `Ctrl+C` | Quit |
| `Ctrl+O` | Toggle expand/collapse on focused `ToolBlock` |
| `Ctrl+L` | Clear viewport |
| `Esc` | Cancel active form / deselect |
| `↑` / `↓` | Scroll viewport (when input not focused) |
| `Tab` | Navigate between suggestion options |

### 7. Rendering (Lipgloss)

- Adaptive borders (`lipgloss.AdaptiveColor`) for light/dark terminal themes
- `TextBlock`: rendered Markdown via `glamour` with terminal-width word wrapping
- `ToolBlock`: visually distinct — thick left border, subtle background tint, monospace result
- `SkillBlock`: similar to tool block but with step progress
- Spinner: dots style during streaming and tool execution
- Error messages: red foreground, no background
- Status bar (optional bottom line): session ID, model name, connection status

### 8. WS Protocol Integration

The TUI uses the existing `clients/ws.Client`:

```go
// Send user message
client.SendMessage("hello")

// Send prompt response
client.SendFrame(ws.Frame{
    Type:   ws.FrameTypeRequest,
    Method: ws.MethodPromptResponse,
    Params: marshal(events.PromptResponsePayload{Value: answer, Token: token}),
})

// Read events (background loop)
for {
    frame, err := client.ReadFrame()
    if err != nil { break }
    if frame.Type == ws.FrameTypeEvent {
        program.Send(Project(frame))
    }
}
```

### 9. Error Handling

- **WS disconnect:** Show reconnection notice, attempt reconnect with backoff
- **Parse errors:** Log to debug, skip malformed frames
- **`assistant.message` with `Error` field:** Display error styled in red, do not crash
- **Tool `failed` status:** Show error inline in tool block, agent continues (tool recovery middleware handles retries)

### 10. Resize

- `tea.WindowSizeMsg` must propagate to viewport and input components
- Viewport: recalculate visible height = `windowHeight - inputHeight - statusBarHeight`
- Glamour: re-render markdown at new width
- Input textarea: adapt width to terminal width minus padding
