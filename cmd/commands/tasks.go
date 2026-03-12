package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/infra/tasks"
	"github.com/dohr-michael/ozzie/pkg/names"
)

// NewTasksCommand returns the tasks subcommand.
func NewTasksCommand() *cli.Command {
	return &cli.Command{
		Name:  "tasks",
		Usage: "Manage async tasks",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all tasks",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "json", Usage: "Output raw JSON"},
				},
				Action: runTasksList,
			},
			{
				Name:      "show",
				Usage:     "Show task details",
				ArgsUsage: "<name|id>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "json", Usage: "Output raw JSON"},
				},
				Action: runTasksShow,
			},
			{
				Name:      "cancel",
				Usage:     "Cancel a task",
				ArgsUsage: "<name|id>",
				Action:    runTasksCancel,
			},
		},
		DefaultCommand: "list",
	}
}

func newTaskStore() *tasks.FileStore {
	return tasks.NewFileStore(filepath.Join(config.OzziePath(), "tasks"))
}

func runTasksList(_ context.Context, cmd *cli.Command) error {
	store := newTaskStore()

	list, err := store.List(tasks.ListFilter{})
	if err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	if cmd.Bool("json") {
		return json.NewEncoder(os.Stdout).Encode(list)
	}

	if len(list) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tPROGRESS\tTITLE")
	for _, t := range list {
		progress := "-"
		if t.Progress.TotalSteps > 0 {
			progress = fmt.Sprintf("%d/%d (%d%%)", t.Progress.CurrentStep, t.Progress.TotalSteps, t.Progress.Percentage)
		} else if t.Status == tasks.TaskCompleted {
			progress = "100%"
		}
		displayName := names.DisplayName(t.ID)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			displayName,
			t.Status,
			progress,
			t.Title,
		)
	}
	return w.Flush()
}

func runTasksShow(_ context.Context, cmd *cli.Command) error {
	ref := cmd.Args().First()
	if ref == "" {
		return fmt.Errorf("usage: ozzie tasks show <name|id>")
	}

	store := newTaskStore()

	t, err := store.Get(ref)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	if cmd.Bool("json") {
		return json.NewEncoder(os.Stdout).Encode(t)
	}

	fmt.Printf("ID:          %s\n", t.ID)
	fmt.Printf("Title:       %s\n", t.Title)
	fmt.Printf("Status:      %s\n", t.Status)
	fmt.Printf("Priority:    %s\n", t.Priority)
	fmt.Printf("Created:     %s\n", t.CreatedAt.Format("2006-01-02 15:04:05"))
	if t.StartedAt != nil {
		fmt.Printf("Started:     %s\n", t.StartedAt.Format("2006-01-02 15:04:05"))
	}
	if t.CompletedAt != nil {
		fmt.Printf("Completed:   %s\n", t.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	if t.SessionID != "" {
		fmt.Printf("Session:     %s\n", t.SessionID)
	}

	if t.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", t.Description)
	}

	// Checkpoints
	cps, _ := store.LoadCheckpoints(t.ID)
	if len(cps) > 0 {
		fmt.Println("\nCheckpoints:")
		for _, cp := range cps {
			fmt.Printf("  [%s] %s: %s\n", cp.Ts.Format("15:04:05"), cp.Type, cp.Summary)
		}
	}

	// Result
	if t.Result != nil && t.Result.Error != "" {
		fmt.Printf("\nError: %s\n", t.Result.Error)
	}

	// Output
	output, _ := store.ReadOutput(t.ID)
	if output != "" {
		fmt.Printf("\nOutput:\n%s\n", output)
	}

	return nil
}

func runTasksCancel(_ context.Context, cmd *cli.Command) error {
	ref := cmd.Args().First()
	if ref == "" {
		return fmt.Errorf("usage: ozzie tasks cancel <name|id>")
	}

	store := newTaskStore()

	t, err := store.Get(ref)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	displayName := names.DisplayName(t.ID)

	if t.Status == tasks.TaskCompleted || t.Status == tasks.TaskCancelled {
		fmt.Printf("Task %s is already %s.\n", displayName, t.Status)
		return nil
	}

	// Direct cancel via store (no gateway connection needed for CLI)
	t.Status = tasks.TaskCancelled
	if err := store.Update(t); err != nil {
		return fmt.Errorf("cancel task: %w", err)
	}

	fmt.Printf("Task %s cancelled.\n", displayName)
	return nil
}
