package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dohr-michael/ozzie/clients/tui/components"
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
			newAPIKeyStep(),
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

	case tea.KeyMsg:
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
func (w *Wizard) View() string {
	if w.done {
		if w.err != nil {
			return components.ErrorStyle.Render(fmt.Sprintf("Error: %v", w.err)) + "\n"
		}
		return w.finalMsg + "\n"
	}
	if w.cancelled {
		return ""
	}
	if w.finalizing {
		return components.HintStyle.Render("  Applying configuration...") + "\n"
	}

	var b strings.Builder

	// Title
	title := components.WelcomeTitleStyle.Render("  Ozzie Setup")
	b.WriteString(title)
	b.WriteString("\n")

	// Progress bar (skip for welcome step)
	if w.currentStep > 0 {
		totalVisible := w.countVisibleSteps()
		currentVisible := w.currentVisibleIndex()
		progress := fmt.Sprintf("  Step %d/%d — %s", currentVisible, totalVisible, w.steps[w.currentStep].Title())
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

	hints := []string{"ctrl+c=quit"}
	if w.currentStep > 0 {
		hints = append([]string{"esc=back"}, hints...)
	}
	hintBar := lipgloss.NewStyle().Foreground(components.Muted).Render("  " + strings.Join(hints, " • "))
	b.WriteString(hintBar)

	return b.String()
}

// initCurrentStep initializes the current step with answers.
func (w *Wizard) initCurrentStep() tea.Cmd {
	if w.currentStep >= len(w.steps) {
		return nil
	}
	return w.steps[w.currentStep].Init(w.answers)
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
