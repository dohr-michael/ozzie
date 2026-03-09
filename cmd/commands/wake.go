package commands

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/dohr-michael/ozzie/internal/ui/setup_wizard"
	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/i18n"
	_ "github.com/dohr-michael/ozzie/internal/ui/components" // register component translations
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
	i18n.Lang = i18n.Detect()
	w := setup_wizard.New()
	p := tea.NewProgram(w)

	model, err := p.Run()
	if err != nil {
		return fmt.Errorf("wizard: %w", err)
	}

	wiz, ok := model.(*setup_wizard.Wizard)
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
