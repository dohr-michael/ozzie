package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/heartbeat"
)

// NewStatusCommand returns the status subcommand.
func NewStatusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Show Ozzie gateway status",
		Action: func(_ context.Context, _ *cli.Command) error {
			hbPath := filepath.Join(config.OzziePath(), "heartbeat.json")
			status, hb, err := heartbeat.Check(hbPath, 2*time.Minute)
			if err != nil {
				return fmt.Errorf("check heartbeat: %w", err)
			}

			switch status {
			case heartbeat.StatusAlive:
				fmt.Printf("Gateway: ALIVE (PID %d, uptime %s)\n", hb.PID, hb.Uptime)
			case heartbeat.StatusStale:
				fmt.Printf("Gateway: STALE (PID %d, last heartbeat %s ago)\n",
					hb.PID, time.Since(hb.Timestamp).Truncate(time.Second))
			case heartbeat.StatusDead:
				fmt.Println("Gateway: NOT RUNNING")
			}

			return nil
		},
	}
}
