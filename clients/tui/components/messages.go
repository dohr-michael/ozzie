// Package components provides reusable TUI components.
package components

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
