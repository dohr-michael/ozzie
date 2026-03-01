package components

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// InputMode represents the current input mode.
type InputMode int

const (
	ModeChat    InputMode = iota // Free text input (default chat)
	ModeText                     // Text prompt with question
	ModeSelect                   // Single selection
	ModeMulti                    // Multiple selection
	ModeConfirm                  // Yes/No confirmation
)

// InputOption represents a selectable option.
type InputOption struct {
	Value       string
	Label       string
	Description string
}

// InputResult is emitted when input is submitted.
type InputResult struct {
	Mode        InputMode
	Text        string   // For ModeChat, ModeText
	Selected    string   // For ModeSelect
	MultiSelect []string // For ModeMulti
	Confirmed   bool     // For ModeConfirm
	Cancelled   bool
	// Context for routing the response
	Field       string // Field name (for prompts)
	ResumeToken string // For resuming agent
}

// CompletedField represents a completed workflow field answer.
type CompletedField struct {
	Label string
	Value string
}

// PromptConfig holds configuration for a prompt.
type PromptConfig struct {
	Question    string        // The question/label to display
	Field       string        // Field name for routing
	Placeholder string        // Placeholder text (for text input)
	Default     string        // Default value
	Required    bool          // Whether the field is required
	Validation  string        // Regex validation pattern
	Options     []InputOption // Options (for select/multi)
	ResumeToken string        // Token for resuming agent
}

// InputZone is a unified input component that handles all input modes.
type InputZone struct {
	mode InputMode

	// Common state
	width       int
	height      int
	disabled    bool
	question    string // Optional question/label
	field       string // Field name for prompts
	resumeToken string
	required    bool

	// Text input
	textInput  textinput.Model
	validation *regexp.Regexp
	errorMsg   string

	// Select/Multi state
	options     []InputOption
	selectIdx   int
	multiSelect map[int]bool

	// Completed fields (for workflow display)
	completedFields []CompletedField

	// Confirm state (uses selectIdx: 0=Yes, 1=No)
}

// NewInputZone creates a new input zone.
func NewInputZone() *InputZone {
	ti := textinput.New()
	ti.Prompt = "" // We render our own ❯ prompt
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 2000
	ti.Width = 80

	return &InputZone{
		mode:        ModeChat,
		textInput:   ti,
		multiSelect: make(map[int]bool),
	}
}

// Init initializes the component.
func (z *InputZone) Init() tea.Cmd {
	return z.textInput.Focus()
}

// Update handles messages.
func (z *InputZone) Update(msg tea.Msg) (*InputZone, tea.Cmd) {
	if z.disabled && z.mode == ModeChat {
		return z, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Drop unparsed SGR mouse escape fragments (e.g. "[<64;75;23M")
		if msg.Type == tea.KeyRunes {
			s := string(msg.Runes)
			if len(s) >= 3 && s[0] == '[' && s[1] == '<' {
				return z, nil
			}
		}

		switch msg.String() {
		case "enter":
			return z.submit()
		case "esc":
			if z.mode != ModeChat {
				return z.cancel()
			}
		case "up":
			if z.mode == ModeSelect || z.mode == ModeMulti || z.mode == ModeConfirm {
				if z.selectIdx > 0 {
					z.selectIdx--
				}
			}
		case "down":
			if z.mode == ModeSelect || z.mode == ModeMulti || z.mode == ModeConfirm {
				maxIdx := len(z.options) - 1
				if z.mode == ModeConfirm {
					maxIdx = 1
				}
				if z.selectIdx < maxIdx {
					z.selectIdx++
				}
			}
		case " ":
			if z.mode == ModeMulti {
				z.multiSelect[z.selectIdx] = !z.multiSelect[z.selectIdx]
			}
		case "y", "Y":
			if z.mode == ModeConfirm {
				z.selectIdx = 0
				return z.submit()
			}
		case "n", "N":
			if z.mode == ModeConfirm {
				z.selectIdx = 1
				return z.submit()
			}
		}

		// Update text input for text modes (only for key messages)
		if z.mode == ModeChat || z.mode == ModeText {
			var cmd tea.Cmd
			z.textInput, cmd = z.textInput.Update(msg)
			z.validate()
			return z, cmd
		}

	// Component messages - preferred way to update state
	case InputSetDisabledMsg:
		z.disabled = msg.Disabled

	case InputPromptConfirmMsg:
		z.PromptConfirm(msg.Question, msg.ResumeToken)

	case InputPromptTextMsg:
		z.PromptText(msg.Question, msg.Field, msg.Placeholder, msg.Default, msg.Required, msg.Validation)

	case InputPromptSelectMsg:
		z.PromptSelect(msg.Question, msg.Field, "", msg.Options, msg.Default)

	case InputPromptMultiMsg:
		z.PromptMulti(msg.Question, msg.Field, "", msg.Options)

	case InputResetMsg:
		z.Reset()
	}

	return z, nil
}

// submit processes the input and returns result.
func (z *InputZone) submit() (*InputZone, tea.Cmd) {
	result := InputResult{
		Mode:        z.mode,
		Field:       z.field,
		ResumeToken: z.resumeToken,
	}

	switch z.mode {
	case ModeChat:
		text := strings.TrimSpace(z.textInput.Value())
		if text == "" {
			return z, nil
		}
		result.Text = text
		z.textInput.SetValue("")
		return z, func() tea.Msg { return result }

	case ModeText:
		text := strings.TrimSpace(z.textInput.Value())
		if text == "" && z.required {
			z.errorMsg = "This field is required"
			return z, nil
		}
		if !z.validate() {
			return z, nil
		}
		result.Text = text
		// Don't reset here - let the caller manage state for workflow fields
		// Reset clears completedFields which we need to preserve
		z.textInput.SetValue("")
		z.errorMsg = ""
		return z, func() tea.Msg { return result }

	case ModeSelect:
		if z.selectIdx >= 0 && z.selectIdx < len(z.options) {
			result.Selected = z.options[z.selectIdx].Value
		}
		// Don't reset - preserve completedFields for workflow
		return z, func() tea.Msg { return result }

	case ModeMulti:
		var selected []string
		for i, opt := range z.options {
			if z.multiSelect[i] {
				selected = append(selected, opt.Value)
			}
		}
		result.MultiSelect = selected
		// Don't reset - preserve completedFields for workflow
		return z, func() tea.Msg { return result }

	case ModeConfirm:
		result.Confirmed = z.selectIdx == 0
		// Reset is OK for confirm since it's not part of a multi-field workflow
		z.Reset()
		return z, func() tea.Msg { return result }
	}

	return z, nil
}

// cancel cancels the current prompt.
func (z *InputZone) cancel() (*InputZone, tea.Cmd) {
	result := InputResult{
		Mode:        z.mode,
		Field:       z.field,
		ResumeToken: z.resumeToken,
		Cancelled:   true,
	}
	z.Reset()
	return z, func() tea.Msg { return result }
}

// validate checks the current text input.
func (z *InputZone) validate() bool {
	if z.validation == nil {
		z.errorMsg = ""
		return true
	}

	value := z.textInput.Value()
	if value == "" {
		z.errorMsg = ""
		return true
	}

	if !z.validation.MatchString(value) {
		z.errorMsg = "Invalid format"
		return false
	}

	z.errorMsg = ""
	return true
}

// View renders the input zone.
func (z *InputZone) View() string {
	if z.disabled && z.mode == ModeChat {
		return z.renderDisabled()
	}

	switch z.mode {
	case ModeChat:
		return z.renderChat()
	case ModeText:
		return z.renderText()
	case ModeSelect:
		return z.renderSelect()
	case ModeMulti:
		return z.renderMulti()
	case ModeConfirm:
		return z.renderConfirm()
	}

	return ""
}

// separator returns a full-width ─── line.
func (z *InputZone) separator() string {
	w := z.width
	if w <= 0 {
		w = 80
	}
	return InputSeparatorStyle.Render(strings.Repeat("─", w))
}

// wrapWithSeparators wraps content between two separator lines.
func (z *InputZone) wrapWithSeparators(content string) string {
	sep := z.separator()
	return sep + "\n" + content + "\n" + sep
}

func (z *InputZone) renderDisabled() string {
	return z.wrapWithSeparators(DisabledStyle.Render("  Waiting for response..."))
}

func (z *InputZone) renderChat() string {
	sep := z.separator()
	return sep + "\n" + InputPromptCharStyle.Render("❯ ") + z.textInput.View() + "\n" + sep
}

// renderCompletedFields renders the completed workflow fields inline.
func (z *InputZone) renderCompletedFields() string {
	if len(z.completedFields) == 0 {
		return ""
	}

	var b strings.Builder
	for _, f := range z.completedFields {
		b.WriteString(AnswerLabelStyle.Render("> " + f.Label + ": "))
		b.WriteString(AnswerStyle.Render(f.Value))
		b.WriteString("\n")
	}
	return b.String()
}

// renderQuestionHeader renders the question/label with optional required marker.
func (z *InputZone) renderQuestionHeader() string {
	if z.question == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString(LabelStyle.Render(z.question))
	if z.required {
		b.WriteString(RequiredStyle.Render("*"))
	}
	b.WriteString(LabelStyle.Render(":"))
	b.WriteString("\n")
	return b.String()
}

// renderHint renders a hint line at the bottom.
func (z *InputZone) renderHint(hint string) string {
	return HintStyle.Render(hint)
}

func (z *InputZone) renderText() string {
	var b strings.Builder

	b.WriteString(z.renderCompletedFields())
	b.WriteString(z.renderQuestionHeader())

	// Input with prompt
	b.WriteString(InputPromptCharStyle.Render("❯ "))
	b.WriteString(z.textInput.View())

	// Error
	if z.errorMsg != "" {
		b.WriteString("\n")
		b.WriteString(ErrorStyle.Render("  " + z.errorMsg))
	}

	b.WriteString("\n")
	b.WriteString(z.renderHint("enter=submit • esc=cancel"))

	return z.wrapWithSeparators(b.String())
}

func (z *InputZone) renderSelect() string {
	var b strings.Builder

	b.WriteString(z.renderCompletedFields())
	b.WriteString(z.renderQuestionHeader())

	// Options
	for i, opt := range z.options {
		if i == z.selectIdx {
			b.WriteString(SelectedOptionStyle.Render("> " + opt.Label))
		} else {
			b.WriteString(OptionStyle.Render("  " + opt.Label))
		}
		if opt.Description != "" {
			b.WriteString(DescriptionStyle.Render(" - " + opt.Description))
		}
		b.WriteString("\n")
	}

	b.WriteString(z.renderHint("↑↓=navigate • enter=select • esc=cancel"))

	return z.wrapWithSeparators(b.String())
}

func (z *InputZone) renderMulti() string {
	var b strings.Builder

	b.WriteString(z.renderCompletedFields())
	b.WriteString(z.renderQuestionHeader())

	// Options with checkboxes
	for i, opt := range z.options {
		check := "[ ]"
		if z.multiSelect[i] {
			check = "[✓]"
		}

		if i == z.selectIdx {
			b.WriteString(SelectedOptionStyle.Render(fmt.Sprintf("> %s %s", check, opt.Label)))
		} else {
			b.WriteString(OptionStyle.Render(fmt.Sprintf("  %s %s", check, opt.Label)))
		}
		if opt.Description != "" {
			b.WriteString(DescriptionStyle.Render(" - " + opt.Description))
		}
		b.WriteString("\n")
	}

	b.WriteString(z.renderHint("↑↓=navigate • space=toggle • enter=submit • esc=cancel"))

	return z.wrapWithSeparators(b.String())
}

func (z *InputZone) renderConfirm() string {
	var b strings.Builder

	// Question (uses special confirm label style)
	if z.question != "" {
		b.WriteString(ConfirmLabelStyle.Render(z.question))
		b.WriteString("\n")
	}

	// Yes/No options
	options := []string{"Yes", "No"}
	for i, opt := range options {
		if i == z.selectIdx {
			b.WriteString(SelectedOptionStyle.Render("> " + opt))
		} else {
			b.WriteString(OptionStyle.Render("  " + opt))
		}
		b.WriteString("\n")
	}

	b.WriteString(z.renderHint("y/n or ↑↓ + enter • esc=cancel"))

	return z.wrapWithSeparators(b.String())
}

// Reset returns to chat mode.
func (z *InputZone) Reset() {
	z.mode = ModeChat
	z.question = ""
	z.field = ""
	z.resumeToken = ""
	z.required = false
	z.options = nil
	z.selectIdx = 0
	z.multiSelect = make(map[int]bool)
	z.validation = nil
	z.errorMsg = ""
	z.completedFields = nil
	z.textInput.SetValue("")
	z.textInput.Placeholder = "Type a message..."
}

// AddCompletedField records a completed workflow field answer.
func (z *InputZone) AddCompletedField(label, value string) {
	z.completedFields = append(z.completedFields, CompletedField{Label: label, Value: value})
}

// ClearCompletedFields clears all completed field answers.
func (z *InputZone) ClearCompletedFields() {
	z.completedFields = nil
}

// SetSize sets the component size.
func (z *InputZone) SetSize(width, height int) {
	z.width = width
	z.height = height
	z.textInput.Width = width - 4
}

// SetDisabled disables chat input (while waiting for response).
func (z *InputZone) SetDisabled(disabled bool) {
	z.disabled = disabled
}

// Focus focuses the text input.
func (z *InputZone) Focus() tea.Cmd {
	return z.textInput.Focus()
}

// Blur blurs the text input.
func (z *InputZone) Blur() {
	z.textInput.Blur()
}

// Mode returns the current input mode.
func (z *InputZone) Mode() InputMode {
	return z.mode
}

// --- Prompt Setup Methods ---

// PromptText sets up a text prompt.
func (z *InputZone) PromptText(question, field, placeholder, resumeToken string, required bool, validation string) {
	z.mode = ModeText
	z.question = question
	z.field = field
	z.resumeToken = resumeToken
	z.required = required
	z.textInput.SetValue("")
	if placeholder != "" {
		z.textInput.Placeholder = placeholder
	} else {
		z.textInput.Placeholder = "Enter value..."
	}
	if validation != "" {
		z.validation, _ = regexp.Compile(validation)
	}
	z.textInput.Focus()
}

// PromptSelect sets up a select prompt.
func (z *InputZone) PromptSelect(question, field, resumeToken string, options []InputOption, defaultValue string) {
	z.mode = ModeSelect
	z.question = question
	z.field = field
	z.resumeToken = resumeToken
	z.options = options
	z.selectIdx = 0
	// Find default
	for i, opt := range options {
		if opt.Value == defaultValue {
			z.selectIdx = i
			break
		}
	}
	z.textInput.Blur()
}

// PromptMulti sets up a multi-select prompt.
func (z *InputZone) PromptMulti(question, field, resumeToken string, options []InputOption) {
	z.mode = ModeMulti
	z.question = question
	z.field = field
	z.resumeToken = resumeToken
	z.options = options
	z.selectIdx = 0
	z.multiSelect = make(map[int]bool)
	z.textInput.Blur()
}

// PromptConfirm sets up a confirmation prompt.
func (z *InputZone) PromptConfirm(question, resumeToken string) {
	z.mode = ModeConfirm
	z.question = question
	z.resumeToken = resumeToken
	z.selectIdx = 0 // Default to Yes
	z.textInput.Blur()
}

// Prompt sets up a prompt using the unified PromptConfig.
func (z *InputZone) Prompt(mode InputMode, cfg PromptConfig) {
	z.mode = mode
	z.question = cfg.Question
	z.field = cfg.Field
	z.resumeToken = cfg.ResumeToken
	z.required = cfg.Required

	switch mode {
	case ModeText:
		z.textInput.SetValue("")
		if cfg.Placeholder != "" {
			z.textInput.Placeholder = cfg.Placeholder
		} else {
			z.textInput.Placeholder = "Enter value..."
		}
		if cfg.Validation != "" {
			z.validation, _ = regexp.Compile(cfg.Validation)
		}
		z.textInput.Focus()

	case ModeSelect:
		z.options = cfg.Options
		z.selectIdx = 0
		for i, opt := range cfg.Options {
			if opt.Value == cfg.Default {
				z.selectIdx = i
				break
			}
		}
		z.textInput.Blur()

	case ModeMulti:
		z.options = cfg.Options
		z.selectIdx = 0
		z.multiSelect = make(map[int]bool)
		z.textInput.Blur()

	case ModeConfirm:
		z.selectIdx = 0 // Default to Yes
		z.textInput.Blur()
	}
}
