package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/dohr-michael/ozzie/internal/infra/scheduler"
	"github.com/dohr-michael/ozzie/internal/core/skills"
)

// NewScheduleCommand returns the schedule subcommand.
func NewScheduleCommand() *cli.Command {
	return &cli.Command{
		Name:  "schedule",
		Usage: "View scheduled skills and trigger history",
		Commands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List skills with cron or event triggers",
				Action: runScheduleList,
			},
			{
				Name:   "history",
				Usage:  "Show recent schedule trigger events",
				Action: runScheduleHistory,
			},
		},
		DefaultCommand: "list",
	}
}

func runScheduleList(_ context.Context, _ *cli.Command) error {
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		cfg = &config.Config{}
	}

	reg := skills.NewRegistry()
	for _, dir := range cfg.Skills.Dirs {
		if err := reg.LoadDir(dir); err != nil {
			continue
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SOURCE\tNAME\tCRON\tEVENT\tENABLED")

	found := false
	for _, sk := range reg.All() {
		if !sk.Triggers.HasScheduleTrigger() {
			continue
		}
		found = true

		cronStr := "-"
		if sk.Triggers.Cron != "" {
			cronStr = sk.Triggers.Cron
		}

		eventStr := "-"
		if sk.Triggers.OnEvent != nil {
			eventStr = sk.Triggers.OnEvent.Event
		}

		fmt.Fprintf(w, "skill\t%s\t%s\t%s\tyes\n", sk.Name, cronStr, eventStr)
	}

	// Dynamic schedules from persistent store
	schedulesDir := filepath.Join(config.OzziePath(), "schedules")
	schedStore := scheduler.NewScheduleStore(schedulesDir)
	entries, err := schedStore.List()
	if err == nil {
		for _, e := range entries {
			found = true
			cronStr := "-"
			if e.CronSpec != "" {
				cronStr = e.CronSpec
			}
			eventStr := "-"
			if e.OnEvent != nil {
				eventStr = e.OnEvent.Event
			}
			enabledStr := "yes"
			if !e.Enabled {
				enabledStr = "no"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", e.Source, e.Title, cronStr, eventStr, enabledStr)
		}
	}

	if !found {
		fmt.Println("No scheduled skills found.")
		return nil
	}

	return w.Flush()
}

func runScheduleHistory(_ context.Context, _ *cli.Command) error {
	logsDir := filepath.Join(config.OzziePath(), "logs")
	logFile := filepath.Join(logsDir, "_global.jsonl")

	f, err := os.Open(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No trigger history found.")
			return nil
		}
		return fmt.Errorf("read history: %w", err)
	}
	defer f.Close()

	// Collect schedule trigger events (keep last 20)
	var triggers []events.Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e events.Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if e.Type != events.EventScheduleTrigger {
			continue
		}
		triggers = append(triggers, e)
		if len(triggers) > 20 {
			triggers = triggers[1:]
		}
	}

	if len(triggers) == 0 {
		fmt.Println("No trigger history found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tSKILL\tTRIGGER\tTASK")
	for _, e := range triggers {
		skillName, _ := e.Payload["skill_name"].(string)
		trigger, _ := e.Payload["trigger"].(string)
		taskID, _ := e.Payload["task_id"].(string)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			e.Timestamp.Format("2006-01-02 15:04:05"),
			skillName, trigger, taskID)
	}
	return w.Flush()
}
