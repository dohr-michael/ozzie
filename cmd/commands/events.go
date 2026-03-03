package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v3"
)

// NewEventsCommand returns the events subcommand.
func NewEventsCommand() *cli.Command {
	return &cli.Command{
		Name:  "events",
		Usage: "Query recent events from the running gateway",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "gateway",
				Usage: "Gateway HTTP base URL",
				Value: "http://127.0.0.1:18420",
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "Maximum number of events to fetch",
				Value: 50,
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Output raw JSON",
			},
			&cli.StringFlag{
				Name:  "type",
				Usage: "Filter by event type",
			},
			&cli.StringFlag{
				Name:    "session",
				Aliases: []string{"s"},
				Usage:   "Filter by session ID",
			},
		},
		Action: runEvents,
	}
}

type eventJSON struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id,omitempty"`
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Source    string         `json:"source"`
	Payload   map[string]any `json:"payload"`
}

func runEvents(_ context.Context, cmd *cli.Command) error {
	gatewayURL := strings.TrimRight(cmd.String("gateway"), "/")
	limit := cmd.Int("limit")
	jsonOut := cmd.Bool("json")
	typeFilter := cmd.String("type")
	sessionFilter := cmd.String("session")

	url := fmt.Sprintf("%s/api/events?limit=%d", gatewayURL, limit)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway returned %s", resp.Status)
	}

	var events []eventJSON
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return fmt.Errorf("decode events: %w", err)
	}

	// Apply client-side filters.
	filtered := events[:0]
	for _, e := range events {
		if typeFilter != "" && e.Type != typeFilter {
			continue
		}
		if sessionFilter != "" && e.SessionID != sessionFilter {
			continue
		}
		filtered = append(filtered, e)
	}

	if jsonOut {
		return json.NewEncoder(os.Stdout).Encode(filtered)
	}

	if len(filtered) == 0 {
		fmt.Println("No events found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tTYPE\tSESSION\tSUMMARY")
	for _, e := range filtered {
		ts := e.Timestamp
		if t, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil {
			ts = t.Format("15:04:05.000")
		}
		sid := e.SessionID
		if len(sid) > 8 {
			sid = sid[:8]
		}
		if sid == "" {
			sid = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ts, e.Type, sid, eventSummary(e))
	}
	return w.Flush()
}

// eventSummary extracts a readable one-liner from an event payload.
func eventSummary(e eventJSON) string {
	if msg, ok := e.Payload["content"].(string); ok {
		if len(msg) > 60 {
			return msg[:60] + "..."
		}
		return msg
	}
	if name, ok := e.Payload["tool"].(string); ok {
		return "tool=" + name
	}
	if label, ok := e.Payload["label"].(string); ok {
		return label
	}
	if errMsg, ok := e.Payload["error"].(string); ok {
		return "error: " + errMsg
	}
	return ""
}
