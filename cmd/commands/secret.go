package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v3"
	"golang.org/x/term"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/secrets"
)

// NewSecretCommand returns the secret management subcommand.
func NewSecretCommand() *cli.Command {
	return &cli.Command{
		Name:  "secret",
		Usage: "Manage encrypted secrets in .env",
		Commands: []*cli.Command{
			{
				Name:      "set",
				Usage:     "Set an encrypted secret (interactive prompt or piped stdin)",
				ArgsUsage: "<KEY>",
				Action:    runSecretSet,
			},
			{
				Name:   "list",
				Usage:  "List all keys and their encryption status",
				Action: runSecretList,
			},
			{
				Name:      "delete",
				Usage:     "Delete a secret from .env",
				ArgsUsage: "<KEY>",
				Action:    runSecretDelete,
			},
			{
				Name:   "rotate",
				Usage:  "Rotate the encryption key and re-encrypt all secrets",
				Action: runSecretRotate,
			},
		},
	}
}

func runSecretSet(_ context.Context, cmd *cli.Command) error {
	key := cmd.Args().First()
	if key == "" {
		return fmt.Errorf("usage: ozzie secret set <KEY>")
	}

	// Load keyring
	kr, err := secrets.NewKeyRing()
	if err != nil {
		return fmt.Errorf("load keyring: %w (run 'ozzie wake' first)", err)
	}
	if kr == nil {
		return fmt.Errorf("no encryption keys found (run 'ozzie wake' first)")
	}

	// Read secret value: interactive terminal or piped stdin
	var value string
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Printf("Enter value for %s: ", key)
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // newline after hidden input
		if err != nil {
			return fmt.Errorf("read secret: %w", err)
		}
		value = string(raw)
	} else {
		// Piped input
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		value = strings.TrimRight(string(data), "\n\r")
	}

	if value == "" {
		return fmt.Errorf("empty value, nothing to store")
	}

	// Encrypt
	encrypted, err := secrets.Encrypt(value, kr.CurrentRecipient())
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	// Write to .env
	dotenvPath := config.DotenvPath()
	if err := secrets.SetEntry(dotenvPath, key, encrypted); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}

	fmt.Printf("Secret %s encrypted and stored in %s\n", key, dotenvPath)
	return nil
}

func runSecretList(_ context.Context, _ *cli.Command) error {
	dotenvPath := config.DotenvPath()
	keys, err := secrets.ListAllKeys(dotenvPath)
	if err != nil {
		return fmt.Errorf("list keys: %w", err)
	}

	if len(keys) == 0 {
		fmt.Println("No entries in .env")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tSTATUS")
	for _, key := range keys {
		val, ok := secrets.GetValue(dotenvPath, key)
		status := "plaintext"
		if ok && secrets.IsEncrypted(val) {
			status = "encrypted"
		}
		fmt.Fprintf(w, "%s\t%s\n", key, status)
	}
	return w.Flush()
}

func runSecretDelete(_ context.Context, cmd *cli.Command) error {
	key := cmd.Args().First()
	if key == "" {
		return fmt.Errorf("usage: ozzie secret delete <KEY>")
	}

	dotenvPath := config.DotenvPath()
	if err := secrets.DeleteEntry(dotenvPath, key); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	fmt.Printf("Deleted %s from %s\n", key, dotenvPath)
	return nil
}

func runSecretRotate(_ context.Context, _ *cli.Command) error {
	ageDir := secrets.AgeDirPath()

	// Ensure .age/ dir exists
	if _, err := os.Stat(ageDir); os.IsNotExist(err) {
		return fmt.Errorf("no .age/ directory found (run 'ozzie wake' first)")
	}

	// Rotate key
	kr, err := secrets.RotateKey(ageDir)
	if err != nil {
		return fmt.Errorf("rotate key: %w", err)
	}

	// Re-encrypt all secrets
	dotenvPath := config.DotenvPath()
	count, err := secrets.ReEncryptAll(dotenvPath, kr)
	if err != nil {
		return fmt.Errorf("re-encrypt: %w", err)
	}

	fmt.Printf("Key rotated. %d secret(s) re-encrypted.\n", count)
	fmt.Println("Restart the gateway to use the new key: ozzie gateway")
	return nil
}
