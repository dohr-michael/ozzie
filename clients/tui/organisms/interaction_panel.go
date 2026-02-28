package organisms

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dohr-michael/ozzie/clients/tui/molecules"
	"github.com/dohr-michael/ozzie/internal/events"
)

// InteractionPanel manages user input and form prompts.
type InteractionPanel struct {
	input           molecules.CommandInput
	form            Form
	pendingMessages []string
}

// NewInteractionPanel creates a new interaction panel.
func NewInteractionPanel(formStyle lipgloss.Style) InteractionPanel {
	return InteractionPanel{
		input: molecules.NewCommandInput(),
		form:  NewForm(formStyle),
	}
}

// SetWidth sets the input width.
func (p *InteractionPanel) SetWidth(w int) {
	p.input.SetWidth(w)
}

// FormActive returns whether the form is active.
func (p *InteractionPanel) FormActive() bool {
	return p.form.Active()
}

// ActivateForm shows the form for a prompt request.
func (p *InteractionPanel) ActivateForm(promptType, label, token string, options []events.PromptOption) {
	p.form.Activate(promptType, label, token, options)
}

// ActivateFormWithOpts shows the form with full options.
func (p *InteractionPanel) ActivateFormWithOpts(opts FormActivateOpts) {
	p.form.ActivateWithOpts(opts)
}

// DeactivateForm hides the form.
func (p *InteractionPanel) DeactivateForm() {
	p.form.Deactivate()
}

// BufferMessage queues a message for later sending.
func (p *InteractionPanel) BufferMessage(content string) {
	p.pendingMessages = append(p.pendingMessages, content)
}

// DrainPending returns and clears all buffered messages.
func (p *InteractionPanel) DrainPending() []string {
	msgs := p.pendingMessages
	p.pendingMessages = nil
	return msgs
}

// UpdateForm routes a message to the form.
func (p InteractionPanel) UpdateForm(msg tea.Msg) (InteractionPanel, tea.Cmd) {
	var cmd tea.Cmd
	p.form, cmd = p.form.Update(msg)
	return p, cmd
}

// UpdateInput routes a message to the command input.
func (p InteractionPanel) UpdateInput(msg tea.Msg) (InteractionPanel, tea.Cmd) {
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

// Update routes a message to the active sub-component.
func (p InteractionPanel) Update(msg tea.Msg) (InteractionPanel, tea.Cmd) {
	if p.form.Active() {
		return p.UpdateForm(msg)
	}
	return p.UpdateInput(msg)
}

// View renders the form if active, otherwise the input.
func (p InteractionPanel) View() string {
	if p.form.Active() {
		return p.form.View()
	}
	return p.input.View()
}
