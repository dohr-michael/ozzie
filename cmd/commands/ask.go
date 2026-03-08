package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"

	"github.com/dohr-michael/ozzie/internal/events"
	wsprotocol "github.com/dohr-michael/ozzie/internal/gateway/ws"
	"github.com/dohr-michael/ozzie/internal/ui/components"

	"context"

	wsclient "github.com/dohr-michael/ozzie/clients/ws"
)

// NewAskCommand returns the ask subcommand.
func NewAskCommand() *cli.Command {
	return &cli.Command{
		Name:      "ask",
		Usage:     "Send a message to the agent and print the response",
		ArgsUsage: "<message>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "gateway",
				Usage: "Gateway WebSocket URL",
				Value: "ws://127.0.0.1:18420/api/ws",
			},
			&cli.StringFlag{
				Name:    "session",
				Aliases: []string{"s"},
				Usage:   "Session ID to resume (empty = new session)",
			},
			&cli.BoolFlag{
				Name:    "dangerously-accept-all",
				Aliases: []string{"y"},
				Usage:   "Auto-approve all dangerous tool executions (no confirmation prompts)",
			},
			&cli.IntFlag{
				Name:  "timeout",
				Usage: "Response timeout in seconds",
				Value: 120,
			},
			&cli.StringFlag{
				Name:    "working-dir",
				Aliases: []string{"w"},
				Usage:   "Working directory for the session (default: current directory)",
			},
			&cli.BoolFlag{
				Name:  "insecure",
				Usage: "Skip authentication (for dev mode)",
			},
		},
		Action: runAsk,
	}
}

func runAsk(_ context.Context, cmd *cli.Command) error {
	message := cmd.Args().First()
	if message == "" {
		return fmt.Errorf("usage: ozzie ask <message>")
	}

	gatewayURL := cmd.String("gateway")
	sessionFlag := cmd.String("session")
	acceptAll := cmd.Bool("dangerously-accept-all")

	workDir := cmd.String("working-dir")
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Disable ANSI colors when stdout is not a TTY (pipe, redirect).
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	// Auto-discover local token for authentication
	var dialOpts []wsclient.DialOption
	if !cmd.Bool("insecure") {
		if token := wsclient.DiscoverLocalToken(); token != "" {
			dialOpts = append(dialOpts, wsclient.WithToken(token))
		}
	}

	timeoutSecs := cmd.Int("timeout")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	client, err := wsclient.Dial(ctx, gatewayURL, dialOpts...)
	if err != nil {
		return fmt.Errorf("connect to gateway: %w", err)
	}
	defer client.Close()

	sid, err := client.OpenSession(wsclient.OpenSessionOpts{
		SessionID: sessionFlag,
		RootDir:   workDir,
	})
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	if sessionFlag == "" {
		fmt.Fprintf(os.Stderr, "session: %s\n", sid)
	}

	if acceptAll {
		if err := client.AcceptAllTools(); err != nil {
			return fmt.Errorf("accept all tools: %w", err)
		}
	}

	if err := client.SendMessage(message); err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return streamLoop(client, acceptAll)
}

// streamLoop reads WS frames synchronously and dispatches to stdout/stderr.
func streamLoop(client *wsclient.Client, acceptAll bool) error {
	width := termWidth()
	wasStreaming := false
	activeTools := make(map[string]*components.ToolCall)

	for {
		frame, err := client.ReadFrame()
		if err != nil {
			return fmt.Errorf("read frame: %w", err)
		}

		if frame.Event == "" {
			continue
		}

		switch events.EventType(frame.Event) {
		case events.EventAssistantStream:
			streamed, err := handleStreamEvent(frame)
			if err != nil {
				return err
			}
			if streamed {
				wasStreaming = true
			}

		case events.EventAssistantMessage:
			return handleAssistantMessage(frame, wasStreaming, width)

		case events.EventToolCall:
			handleToolCall(frame, activeTools, width)

		case events.EventPromptRequest:
			if err := handlePrompt(frame, client, acceptAll); err != nil {
				return err
			}

		case events.EventSkillStarted:
			handleSkillStarted(frame)

		case events.EventSkillCompleted:
			handleSkillCompleted(frame)

		case events.EventSkillStepStarted:
			handleSkillStepStarted(frame)

		case events.EventSkillStepCompleted:
			handleSkillStepCompleted(frame)
		}
	}
}

// handleStreamEvent prints stream deltas to stdout. Returns true if content was printed.
func handleStreamEvent(frame wsprotocol.Frame) (bool, error) {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return false, nil
	}
	payload, ok := events.GetAssistantStreamPayload(evt)
	if !ok {
		return false, nil
	}
	if payload.Phase == events.StreamPhaseDelta && payload.Content != "" {
		fmt.Print(payload.Content)
		return true, nil
	}
	return false, nil
}

// handleAssistantMessage finalizes the response.
func handleAssistantMessage(frame wsprotocol.Frame, wasStreaming bool, width int) error {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return nil
	}
	payload, ok := events.GetAssistantMessagePayload(evt)
	if !ok {
		return nil
	}

	if payload.Error != "" {
		if wasStreaming {
			fmt.Println()
		}
		fmt.Fprintln(os.Stderr, components.RenderError(payload.Error, width))
		return fmt.Errorf("agent error: %s", payload.Error)
	}

	if wasStreaming {
		// Stream deltas were already printed; just add a final newline.
		fmt.Println()
	} else if payload.Content != "" {
		// Non-streamed response (fallback): print the full content.
		fmt.Println(payload.Content)
	}

	return nil
}

// handleToolCall tracks tool calls and renders them on stderr.
func handleToolCall(frame wsprotocol.Frame, activeTools map[string]*components.ToolCall, width int) {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return
	}
	payload, ok := events.GetToolCallPayload(evt)
	if !ok {
		return
	}

	switch payload.Status {
	case events.ToolStatusStarted:
		tc := &components.ToolCall{
			Name:      payload.Name,
			Arguments: components.FormatArgumentsCompact(payload.Arguments),
			Status:    components.ToolStatusRunning,
		}
		activeTools[payload.Name] = tc
		fmt.Fprintln(os.Stderr, components.RenderToolResult(*tc, width))

	case events.ToolStatusCompleted:
		tc, exists := activeTools[payload.Name]
		if !exists {
			tc = &components.ToolCall{Name: payload.Name}
		}
		tc.Completed = true
		tc.Result = payload.Result
		tc.Status = components.ToolStatusCompleted
		tc.Arguments = components.FormatArgumentsCompact(payload.Arguments)
		fmt.Fprintln(os.Stderr, components.RenderToolResult(*tc, width))
		delete(activeTools, payload.Name)

	case events.ToolStatusFailed:
		tc, exists := activeTools[payload.Name]
		if !exists {
			tc = &components.ToolCall{Name: payload.Name}
		}
		tc.Completed = true
		tc.Status = components.ToolStatusFailed
		if payload.Error != "" {
			tc.Error = fmt.Errorf("%s", payload.Error)
		}
		fmt.Fprintln(os.Stderr, components.RenderToolResult(*tc, width))
		delete(activeTools, payload.Name)
	}
}

// handlePrompt handles interactive prompts from the gateway.
func handlePrompt(frame wsprotocol.Frame, client *wsclient.Client, acceptAll bool) error {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return nil
	}
	payload, ok := events.GetPromptRequestPayload(evt)
	if !ok {
		return nil
	}

	// Auto-approve mode: confirm without user interaction.
	if acceptAll {
		return client.RespondToPrompt(payload.Token, false)
	}

	scanner := bufio.NewScanner(os.Stdin)

	switch payload.Type {
	case events.PromptTypeConfirm:
		fmt.Fprintf(os.Stderr, "%s [y/N] ", payload.Label)
		if scanner.Scan() {
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			cancelled := answer != "y" && answer != "yes"
			return client.RespondToPrompt(payload.Token, cancelled)
		}
		return client.RespondToPrompt(payload.Token, true)

	case events.PromptTypeText, events.PromptTypePassword:
		fmt.Fprintf(os.Stderr, "%s: ", payload.Label)
		if scanner.Scan() {
			return client.RespondToPromptWithValue(payload.Token, scanner.Text())
		}
		return client.RespondToPrompt(payload.Token, true)

	case events.PromptTypeSelect:
		fmt.Fprintln(os.Stderr, payload.Label)
		for i, opt := range payload.Options {
			label := opt.Label
			if label == "" {
				label = opt.Value
			}
			fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, label)
		}
		fmt.Fprintf(os.Stderr, "Choice [1-%d]: ", len(payload.Options))
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			var idx int
			if _, err := fmt.Sscanf(input, "%d", &idx); err == nil && idx >= 1 && idx <= len(payload.Options) {
				return client.RespondToPromptWithValue(payload.Token, payload.Options[idx-1].Value)
			}
		}
		return client.RespondToPrompt(payload.Token, true)

	case events.PromptTypeMulti:
		fmt.Fprintln(os.Stderr, payload.Label)
		for i, opt := range payload.Options {
			label := opt.Label
			if label == "" {
				label = opt.Value
			}
			fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, label)
		}
		fmt.Fprintf(os.Stderr, "Choices (comma-separated, e.g. 1,3): ")
		if scanner.Scan() {
			parts := strings.Split(scanner.Text(), ",")
			var values []string
			for _, p := range parts {
				var idx int
				if _, err := fmt.Sscanf(strings.TrimSpace(p), "%d", &idx); err == nil && idx >= 1 && idx <= len(payload.Options) {
					values = append(values, payload.Options[idx-1].Value)
				}
			}
			if len(values) > 0 {
				return client.RespondToPromptWithValues(payload.Token, values)
			}
		}
		return client.RespondToPrompt(payload.Token, true)

	default:
		// Unknown prompt type — cancel.
		return client.RespondToPrompt(payload.Token, true)
	}
}

// Skill event handlers — informational output on stderr.

func handleSkillStarted(frame wsprotocol.Frame) {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return
	}
	payload, ok := events.GetSkillStartedPayload(evt)
	if !ok {
		return
	}
	fmt.Fprintf(os.Stderr, "▶ skill: %s\n", payload.SkillName)
}

func handleSkillCompleted(frame wsprotocol.Frame) {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return
	}
	payload, ok := events.GetSkillCompletedPayload(evt)
	if !ok {
		return
	}
	if payload.Error != "" {
		fmt.Fprintf(os.Stderr, "✗ skill %s: %s\n", payload.SkillName, payload.Error)
	} else {
		fmt.Fprintf(os.Stderr, "✓ skill %s (%s)\n", payload.SkillName, payload.Duration)
	}
}

func handleSkillStepStarted(frame wsprotocol.Frame) {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return
	}
	payload, ok := events.GetSkillStepStartedPayload(evt)
	if !ok {
		return
	}
	fmt.Fprintf(os.Stderr, "  ▸ %s/%s\n", payload.SkillName, payload.StepTitle)
}

func handleSkillStepCompleted(frame wsprotocol.Frame) {
	var evt events.Event
	if err := json.Unmarshal(frame.Payload, &evt); err != nil {
		return
	}
	payload, ok := events.GetSkillStepCompletedPayload(evt)
	if !ok {
		return
	}
	if payload.Error != "" {
		fmt.Fprintf(os.Stderr, "  ✗ %s/%s: %s\n", payload.SkillName, payload.StepID, payload.Error)
	}
}

// termWidth returns the current terminal width, defaulting to 80.
func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stderr.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}
