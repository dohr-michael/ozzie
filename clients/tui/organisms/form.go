package organisms

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dohr-michael/ozzie/internal/events"
)

// FormResponseMsg is sent when the user submits or cancels a form.
type FormResponseMsg struct {
	Token     string
	Cancelled bool
	Value     string
}

// FormActivateOpts holds all parameters for activating a form.
type FormActivateOpts struct {
	PromptType  string
	Label       string
	Token       string
	Options     []events.PromptOption
	HelpText    string
	Placeholder string
	MinSelect   int
	MaxSelect   int
}

// Form handles interactive prompt requests (confirm, text, select, multi, password).
type Form struct {
	active      bool
	token       string
	promptType  string
	label       string
	options     []events.PromptOption
	helpText    string
	placeholder string
	minSelect   int
	maxSelect   int
	cursor      int
	selected    map[int]bool // multi-select state
	textInput   textinput.Model
	style       lipgloss.Style
}

// NewForm creates an inactive form.
func NewForm(style lipgloss.Style) Form {
	ti := textinput.New()
	ti.CharLimit = 256
	return Form{
		style:     style,
		textInput: ti,
		selected:  make(map[int]bool),
	}
}

// Active returns whether a prompt is active.
func (f *Form) Active() bool {
	return f.active
}

// Activate sets up the form for a new prompt request.
func (f *Form) Activate(promptType, label, token string, options []events.PromptOption) {
	f.ActivateWithOpts(FormActivateOpts{
		PromptType: promptType,
		Label:      label,
		Token:      token,
		Options:    options,
	})
}

// ActivateWithOpts sets up the form with full options.
func (f *Form) ActivateWithOpts(opts FormActivateOpts) {
	f.active = true
	f.token = opts.Token
	f.promptType = opts.PromptType
	f.label = opts.Label
	f.options = opts.Options
	f.helpText = opts.HelpText
	f.placeholder = opts.Placeholder
	f.minSelect = opts.MinSelect
	f.maxSelect = opts.MaxSelect
	f.cursor = 0
	f.selected = make(map[int]bool)

	f.textInput.Reset()
	if opts.Placeholder != "" {
		f.textInput.Placeholder = opts.Placeholder
	}
	if opts.PromptType == "password" {
		f.textInput.EchoMode = textinput.EchoPassword
	} else {
		f.textInput.EchoMode = textinput.EchoNormal
	}
	if opts.PromptType == "text" || opts.PromptType == "password" {
		f.textInput.Focus()
	}
}

// Deactivate resets the form.
func (f *Form) Deactivate() {
	f.active = false
	f.textInput.Blur()
}

// Update handles form input.
func (f Form) Update(msg tea.Msg) (Form, tea.Cmd) {
	if !f.active {
		return f, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		if f.promptType == "text" || f.promptType == "password" {
			var cmd tea.Cmd
			f.textInput, cmd = f.textInput.Update(msg)
			return f, cmd
		}
		return f, nil
	}

	switch keyMsg.Type {
	case tea.KeyEsc:
		f.active = false
		return f, func() tea.Msg {
			return FormResponseMsg{Token: f.token, Cancelled: true}
		}
	}

	switch f.promptType {
	case "confirm":
		return f.updateConfirm(keyMsg)
	case "text", "password":
		return f.updateText(keyMsg)
	case "select":
		return f.updateSelect(keyMsg)
	case "multi":
		return f.updateMulti(keyMsg)
	default:
		return f, nil
	}
}

func (f Form) updateConfirm(msg tea.KeyMsg) (Form, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y":
		f.active = false
		return f, func() tea.Msg {
			return FormResponseMsg{Token: f.token, Value: "true"}
		}
	case "n":
		f.active = false
		return f, func() tea.Msg {
			return FormResponseMsg{Token: f.token, Cancelled: true}
		}
	}
	return f, nil
}

func (f Form) updateText(msg tea.KeyMsg) (Form, tea.Cmd) {
	if msg.Type == tea.KeyEnter {
		value := f.textInput.Value()
		f.active = false
		return f, func() tea.Msg {
			return FormResponseMsg{Token: f.token, Value: value}
		}
	}
	var cmd tea.Cmd
	f.textInput, cmd = f.textInput.Update(msg)
	return f, cmd
}

func (f Form) updateSelect(msg tea.KeyMsg) (Form, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if f.cursor > 0 {
			f.cursor--
		}
	case tea.KeyDown:
		if f.cursor < len(f.options)-1 {
			f.cursor++
		}
	case tea.KeyEnter:
		if f.cursor < len(f.options) {
			value := f.options[f.cursor].Value
			f.active = false
			return f, func() tea.Msg {
				return FormResponseMsg{Token: f.token, Value: value}
			}
		}
	}
	return f, nil
}

func (f Form) updateMulti(msg tea.KeyMsg) (Form, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if f.cursor > 0 {
			f.cursor--
		}
	case tea.KeyDown:
		if f.cursor < len(f.options)-1 {
			f.cursor++
		}
	case tea.KeySpace:
		// Toggle selection
		if f.cursor < len(f.options) && !f.options[f.cursor].Disabled {
			if f.selected[f.cursor] {
				delete(f.selected, f.cursor)
			} else {
				// Enforce maxSelect
				if f.maxSelect <= 0 || len(f.selected) < f.maxSelect {
					f.selected[f.cursor] = true
				}
			}
		}
	case tea.KeyEnter:
		// Enforce minSelect
		if f.minSelect > 0 && len(f.selected) < f.minSelect {
			return f, nil // don't submit yet
		}
		// Collect selected values as JSON array
		var values []string
		for i, opt := range f.options {
			if f.selected[i] {
				values = append(values, opt.Value)
			}
		}
		jsonVal, _ := json.Marshal(values)
		f.active = false
		return f, func() tea.Msg {
			return FormResponseMsg{Token: f.token, Value: string(jsonVal)}
		}
	}
	return f, nil
}

// View renders the form.
func (f Form) View() string {
	if !f.active {
		return ""
	}

	var sb strings.Builder

	// Help text (if present, shown above)
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // gray

	switch f.promptType {
	case "confirm":
		sb.WriteString(fmt.Sprintf("%s [Y/n] ", f.label))
		if f.helpText != "" {
			sb.WriteString("\n" + helpStyle.Render(f.helpText))
		}

	case "text", "password":
		sb.WriteString(f.label + "\n")
		if f.helpText != "" {
			sb.WriteString(helpStyle.Render(f.helpText) + "\n")
		}
		sb.WriteString(f.textInput.View())

	case "select":
		sb.WriteString(f.label + "\n")
		if f.helpText != "" {
			sb.WriteString(helpStyle.Render(f.helpText) + "\n")
		}
		for i, opt := range f.options {
			cursor := "  "
			if i == f.cursor {
				cursor = "> "
			}
			line := cursor + opt.Label
			if opt.Description != "" {
				line += "\n    " + helpStyle.Render(opt.Description)
			}
			sb.WriteString(line + "\n")
		}

	case "multi":
		sb.WriteString(f.label + "\n")
		if f.helpText != "" {
			sb.WriteString(helpStyle.Render(f.helpText) + "\n")
		}
		hint := "  (Space: toggle, Enter: submit"
		if f.minSelect > 0 {
			hint += fmt.Sprintf(", min: %d", f.minSelect)
		}
		if f.maxSelect > 0 {
			hint += fmt.Sprintf(", max: %d", f.maxSelect)
		}
		hint += ")"
		sb.WriteString(helpStyle.Render(hint) + "\n")

		for i, opt := range f.options {
			cursor := "  "
			if i == f.cursor {
				cursor = "> "
			}
			check := "[ ]"
			if f.selected[i] {
				check = "[x]"
			}
			line := cursor + check + " " + opt.Label
			if opt.Disabled {
				line = helpStyle.Render(line + " (disabled)")
			}
			if opt.Description != "" {
				line += "\n      " + helpStyle.Render(opt.Description)
			}
			sb.WriteString(line + "\n")
		}

		if f.minSelect > 0 {
			count := len(f.selected)
			if count < f.minSelect {
				sb.WriteString(helpStyle.Render(fmt.Sprintf("  Select at least %d more", f.minSelect-count)) + "\n")
			}
		}
	}

	return f.style.Render(sb.String())
}
