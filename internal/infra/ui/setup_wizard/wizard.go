package setup_wizard

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dohr-michael/ozzie/internal/infra/i18n"
	"github.com/dohr-michael/ozzie/internal/infra/ui/components"
)

// Wizard is the top-level Bubbletea model that orchestrates wizard steps.
type Wizard struct {
	steps       []Step
	currentStep int
	answers     Answers
	width       int
	height      int
	finalizing  bool
	done        bool
	cancelled   bool
	err         error
	finalMsg    string
}

// New creates a new wizard with all steps.
func New() *Wizard {
	return &Wizard{
		steps: []Step{
			newWelcomeStep(),
			newProviderStep(),
			newDefaultStep(),
			newEmbeddingStep(),
			newLayeredStep(),
			newMCPStep(),
			newGatewayStep(),
			newConfirmStep(),
		},
		answers: make(Answers),
	}
}

// Err returns the error if the wizard failed.
func (w *Wizard) Err() error { return w.err }

// Cancelled returns true if the user cancelled.
func (w *Wizard) Cancelled() bool { return w.cancelled }

// Init initializes the wizard.
func (w *Wizard) Init() tea.Cmd {
	return w.initCurrentStep()
}

// Update handles messages.
func (w *Wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
		w.propagateSize()
		return w, nil

	case finalizeResultMsg:
		if msg.err != nil {
			w.err = msg.err
			w.done = true
			return w, tea.Quit
		}
		w.finalMsg = msg.message
		w.done = true
		return w, tea.Quit

	case StepDoneMsg:
		return w.advanceStep()

	case StepBackMsg:
		return w.goBack()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			w.cancelled = true
			return w, tea.Quit
		case "esc":
			if w.currentStep == 0 {
				w.cancelled = true
				return w, tea.Quit
			}
		}
	}

	// Delegate to current step.
	if w.currentStep < len(w.steps) {
		step, cmd := w.steps[w.currentStep].Update(msg)
		w.steps[w.currentStep] = step
		return w, cmd
	}

	return w, nil
}

// View renders the wizard.
func (w *Wizard) View() tea.View {
	var content string
	if w.done {
		if w.err != nil {
			content = components.ErrorStyle.Render(fmt.Sprintf("Error: %v", w.err)) + "\n"
		} else {
			content = w.finalMsg + "\n"
		}
	} else if w.cancelled {
		content = ""
	} else if w.finalizing {
		content = components.HintStyle.Render(i18n.T("wizard.applying")) + "\n"
	} else {
		var b strings.Builder

		// Title
		title := components.WelcomeTitleStyle.Render(i18n.T("wizard.title"))
		b.WriteString(title)
		b.WriteString("\n")

		// Progress bar (skip for welcome step)
		if w.currentStep > 0 {
			totalVisible := w.countVisibleSteps()
			currentVisible := w.currentVisibleIndex()
			progress := fmt.Sprintf(i18n.T("wizard.step_progress"), currentVisible, totalVisible, w.steps[w.currentStep].Title())
			b.WriteString(components.HintStyle.Render(progress))
			b.WriteString("\n")
		}

		// Separator
		width := w.width
		if width <= 0 {
			width = 80
		}
		b.WriteString(components.InputSeparatorStyle.Render(strings.Repeat("─", width)))
		b.WriteString("\n")

		// Step content
		b.WriteString("\n")
		b.WriteString(w.steps[w.currentStep].View())
		b.WriteString("\n")

		// Bottom separator + hints
		b.WriteString(components.InputSeparatorStyle.Render(strings.Repeat("─", width)))
		b.WriteString("\n")

		hints := []string{i18n.T("hint.quit")}
		if w.currentStep > 0 {
			hints = append([]string{i18n.T("hint.back")}, hints...)
		}
		hintBar := lipgloss.NewStyle().Foreground(components.Muted).Render("  " + strings.Join(hints, " • "))
		b.WriteString(hintBar)
		content = b.String()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// initCurrentStep initializes the current step with answers.
func (w *Wizard) initCurrentStep() tea.Cmd {
	if w.currentStep >= len(w.steps) {
		return nil
	}
	w.propagateSize()
	return w.steps[w.currentStep].Init(w.answers)
}

// propagateSize sends the available content dimensions to Resizable steps.
func (w *Wizard) propagateSize() {
	if w.width == 0 && w.height == 0 {
		return
	}
	h := w.contentHeight()
	for _, s := range w.steps {
		if r, ok := s.(Resizable); ok {
			r.SetSize(w.width, h)
		}
	}
}

// contentHeight returns the height available for step content.
// Chrome: title(1) + progress(1) + separator(1) + padding(1) + separator(1) + hints(1) + padding(1) = 7
func (w *Wizard) contentHeight() int {
	h := w.height - 7
	if h < 10 {
		h = 10
	}
	return h
}

// advanceStep collects answers and moves to the next step.
func (w *Wizard) advanceStep() (tea.Model, tea.Cmd) {
	// Collect answers from current step.
	collected := w.steps[w.currentStep].Collect()
	w.answers.Merge(collected)

	// Check for cancel action from welcome step.
	if w.answers.String("action", "") == "cancel" {
		w.cancelled = true
		return w, tea.Quit
	}

	// Advance to next non-skipped step.
	w.currentStep++
	for w.currentStep < len(w.steps) {
		if !w.steps[w.currentStep].ShouldSkip(w.answers) {
			return w, w.initCurrentStep()
		}
		w.currentStep++
	}

	// All steps done — finalize.
	w.finalizing = true
	return w, w.finalize()
}

// goBack moves to the previous non-skipped step.
func (w *Wizard) goBack() (tea.Model, tea.Cmd) {
	w.currentStep--
	for w.currentStep >= 0 {
		if !w.steps[w.currentStep].ShouldSkip(w.answers) {
			return w, w.initCurrentStep()
		}
		w.currentStep--
	}
	// If we went past the beginning, go to step 0.
	w.currentStep = 0
	return w, w.initCurrentStep()
}

// finalize runs the finalization logic.
func (w *Wizard) finalize() tea.Cmd {
	answers := w.answers
	return func() tea.Msg {
		msg, err := Finalize(answers)
		return finalizeResultMsg{message: msg, err: err}
	}
}

type finalizeResultMsg struct {
	message string
	err     error
}

// countVisibleSteps returns the number of non-welcome, non-skipped steps.
func (w *Wizard) countVisibleSteps() int {
	count := 0
	for i := 1; i < len(w.steps); i++ {
		if !w.steps[i].ShouldSkip(w.answers) {
			count++
		}
	}
	return count
}

// currentVisibleIndex returns the 1-based index among visible steps.
func (w *Wizard) currentVisibleIndex() int {
	idx := 0
	for i := 1; i <= w.currentStep && i < len(w.steps); i++ {
		if !w.steps[i].ShouldSkip(w.answers) {
			idx++
		}
	}
	return idx
}
