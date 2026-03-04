package commands

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/wizard"
)

// NewWakeCommand returns the onboarding subcommand.
func NewWakeCommand() *cli.Command {
	return &cli.Command{
		Name:   "wake",
		Usage:  "Initialize the Ozzie home directory (~/.ozzie)",
		Action: runWake,
	}
}

func runWake(_ context.Context, _ *cli.Command) error {
	w := wizard.New()
	p := tea.NewProgram(w, tea.WithAltScreen())

	model, err := p.Run()
	if err != nil {
		return fmt.Errorf("wizard: %w", err)
	}

	wiz, ok := model.(*wizard.Wizard)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}
	if wiz.Cancelled() {
		fmt.Println("Setup cancelled.")
		return nil
	}
	if wiz.Err() != nil {
		return wiz.Err()
	}

	// Success message already printed by wizard via View().
	return nil
}
