// Command secret_flow exercises the full secret encryption lifecycle via WS.
//
// It connects to a running Ozzie gateway, simulates a password prompt,
// verifies that the response is encrypted, then calls set_secret to
// decrypt and write to .env.
//
// Usage: secret_flow -gateway ws://127.0.0.1:PORT/api/ws -secret TOKEN_VALUE -env-name MY_TOKEN
//
// Exit codes:
//
//	0 = all checks passed
//	1 = a check failed
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	wsclient "github.com/dohr-michael/ozzie/clients/ws"
	"github.com/dohr-michael/ozzie/internal/events"
)

func main() {
	gatewayURL := flag.String("gateway", "ws://127.0.0.1:18420/api/ws", "Gateway WS URL")
	secret := flag.String("secret", "e2e-test-secret-value-42", "Secret value to encrypt")
	envName := flag.String("env-name", "E2E_TEST_SECRET", "Env var name for set_secret")
	timeout := flag.Duration("timeout", 30*time.Second, "Overall timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if err := run(ctx, *gatewayURL, *secret, *envName); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, gatewayURL, secret, envName string) error {
	// ── Step 1: Connect and open session ────────────────────────────────
	client, err := wsclient.Dial(ctx, gatewayURL)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer client.Close()

	sessionID, err := client.OpenSession(wsclient.OpenSessionOpts{})
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	fmt.Printf("CHECK session opened: %s\n", sessionID)

	// ── Step 2: Send a message that asks the agent to store a secret ────
	// The agent should:
	//   a) emit a password prompt to collect the token
	//   b) receive the encrypted value
	//   c) call set_secret to decrypt + store
	msg := fmt.Sprintf(
		`I need to store a secret credential. Please ask me for it using a password prompt, then store it as %s using the set_secret tool. The secret is sensitive — never display it in plaintext.`,
		envName,
	)
	if err := client.SendMessage(msg); err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	fmt.Println("CHECK message sent")

	// ── Step 3: Read frames, wait for password prompt, respond ──────────
	promptAnswered := false
	setSecretCalled := false
	done := false

	for !done {
		frame, err := client.ReadFrame()
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("timeout waiting for frames")
			}
			return fmt.Errorf("read frame: %w", err)
		}

		// Skip non-event frames (responses to our requests)
		if frame.Event == "" {
			continue
		}

		switch events.EventType(frame.Event) {
		case events.EventPromptRequest:
			var evt events.Event
			if err := json.Unmarshal(frame.Payload, &evt); err != nil {
				continue
			}
			payload, ok := events.GetPromptRequestPayload(evt)
			if !ok {
				continue
			}

			fmt.Printf("CHECK prompt received: type=%s label=%q\n", payload.Type, payload.Label)

			if payload.Type == events.PromptTypePassword {
				// Respond with the secret — hub should encrypt it
				if err := client.RespondToPromptWithValue(payload.Token, secret); err != nil {
					return fmt.Errorf("respond to password prompt: %w", err)
				}
				promptAnswered = true
				fmt.Println("CHECK password prompt answered")
			} else {
				// Non-password prompt (e.g. dangerous tool confirmation) → auto-approve
				if err := client.RespondToPrompt(payload.Token, false); err != nil {
					return fmt.Errorf("respond to prompt: %w", err)
				}
			}

		case events.EventToolCall:
			var evt events.Event
			if err := json.Unmarshal(frame.Payload, &evt); err != nil {
				continue
			}
			payload, ok := events.GetToolCallPayload(evt)
			if !ok {
				continue
			}

			if payload.Name == "set_secret" {
				setSecretCalled = true
				fmt.Printf("CHECK set_secret called: status=%s\n", payload.Status)

				// Verify the arguments contain ENC[age:...], not plaintext
				if payload.Status == events.ToolStatusStarted {
					argsJSON, _ := json.Marshal(payload.Arguments)
					argsStr := string(argsJSON)
					if strings.Contains(argsStr, secret) {
						return fmt.Errorf("SECURITY: set_secret arguments contain plaintext secret")
					}
					if strings.Contains(argsStr, "ENC[age:") {
						fmt.Println("CHECK set_secret value is encrypted (ENC[age:...])")
					}
				}
			}

		case events.EventAssistantMessage:
			// Final response — we're done
			fmt.Println("CHECK assistant message received (done)")
			done = true
		}
	}

	// ── Step 4: Verify results ──────────────────────────────────────────
	if !promptAnswered {
		return fmt.Errorf("agent never sent a password prompt")
	}

	if !setSecretCalled {
		return fmt.Errorf("agent never called set_secret")
	}

	fmt.Println("CHECK all flow checks passed")
	return nil
}
