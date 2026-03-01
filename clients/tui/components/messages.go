// Package components provides reusable TUI components.
package components

// ChatStartInteractionMsg starts a new user interaction.
type ChatStartInteractionMsg struct {
	UserMessage string
}

// ChatAddToolCallMsg adds a tool call to the current interaction.
type ChatAddToolCallMsg struct {
	Name      string
	Arguments string
}

// ChatAddToolResultMsg updates the last tool call with its result.
type ChatAddToolResultMsg struct {
	Name   string
	Result string
	Error  error
}

// ChatSetToolAwaitingMsg marks a tool as awaiting user confirmation.
type ChatSetToolAwaitingMsg struct {
	Name      string
	Arguments string
}

// ChatSetStreamingMsg sets the current streaming content.
type ChatSetStreamingMsg struct {
	Content string
}

// ChatCompleteInteractionMsg finishes the current interaction.
type ChatCompleteInteractionMsg struct {
	Message string
	IsError bool
}

// ChatSetThinkingMsg shows/hides the thinking indicator.
type ChatSetThinkingMsg struct {
	Thinking bool
}

// ChatClearMsg removes all messages.
type ChatClearMsg struct{}

// ChatAddCorrelatedMsg adds a message with correlation ID.
type ChatAddCorrelatedMsg struct {
	Content           string
	CorrelationID     string
	ClearsCorrelation bool
}

// HeaderSetStreamingMsg updates the streaming indicator.
type HeaderSetStreamingMsg struct {
	Streaming bool
}

// HeaderAddTokensMsg adds to the token count.
type HeaderAddTokensMsg struct {
	Tokens int
}

// HeaderSetSizeMsg updates the header width.
type HeaderSetSizeMsg struct {
	Width int
}

// InputSetDisabledMsg disables/enables the input.
type InputSetDisabledMsg struct {
	Disabled bool
}

// InputPromptConfirmMsg sets up a confirmation prompt.
type InputPromptConfirmMsg struct {
	Question    string
	ResumeToken string
}

// InputPromptTextMsg sets up a text prompt.
type InputPromptTextMsg struct {
	Question    string
	Field       string
	Placeholder string
	Default     string
	Required    bool
	Validation  string
}

// InputPromptSelectMsg sets up a select prompt.
type InputPromptSelectMsg struct {
	Question string
	Field    string
	Options  []InputOption
	Default  string
}

// InputPromptMultiMsg sets up a multi-select prompt.
type InputPromptMultiMsg struct {
	Question string
	Field    string
	Options  []InputOption
}

// InputResetMsg resets to chat mode.
type InputResetMsg struct{}
