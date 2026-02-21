package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/dohr-michael/ozzie/internal/config"
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
	root := config.OzziePath()
	created := false

	// Ensure directories exist.
	dirs := []string{
		root,
		filepath.Join(root, "logs"),
		filepath.Join(root, "skills"),
		filepath.Join(root, "sessions"),
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); err != nil {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", d, err)
			}
			fmt.Printf("  Created %s\n", d)
			created = true
		}
	}

	// Write default config if missing.
	configPath := config.ConfigPath()
	if _, err := os.Stat(configPath); err != nil {
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0o644); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		fmt.Printf("  Created %s\n", configPath)
		created = true
	}

	// Write default .env if missing.
	dotenvPath := config.DotenvPath()
	if _, err := os.Stat(dotenvPath); err != nil {
		if err := os.WriteFile(dotenvPath, []byte(defaultDotenv), 0o600); err != nil {
			return fmt.Errorf("write .env: %w", err)
		}
		fmt.Printf("  Created %s\n", dotenvPath)
		created = true
	}

	if !created {
		fmt.Printf("Already awake — %s is complete. Nothing to do.\n", root)
		return nil
	}

	fmt.Println(wakeMessage(root))
	return nil
}

const defaultConfig = `{
	// Ozzie Configuration
	// Docs: https://github.com/dohr-michael/ozzie

	"gateway": {
		"host": "127.0.0.1",
		"port": 18420
	},

	"models": {
		"default": "claude",
		"providers": {
			"claude": {
				"driver": "anthropic",
				"model": "claude-sonnet-4-20250514",
				"auth": {
					"env_var": "ANTHROPIC_API_KEY"
				},
				"max_tokens": 4096
			}

			// Local model via Ollama (no auth required)
			// "local": {
			// 	"driver": "ollama",
			// 	"model": "llama3.1:8b",
			// 	"base_url": "http://localhost:11434",
			// 	"max_tokens": 4096
			// }
		}
	},

	"events": {
		"buffer_size": 1024
	},

	"agent": {
		"system_prompt": ""
	}
}
`

const defaultDotenv = `# Ozzie environment variables
# This file is loaded automatically. Existing env vars are never overridden.

# ANTHROPIC_API_KEY=sk-ant-...
# OPENAI_API_KEY=sk-...
`

func wakeMessage(root string) string {
	return fmt.Sprintf(`
  Morning. I'm Ozzie.

  Home set up at %s
  Config, logs, skills, sessions — all in there.

  Next steps:
    1. Drop your API key in %s/.env
    2. Tweak %s/config.jsonc if you feel like it
    3. Run: ozzie gateway

  Let's see what's out there.
`, root, root, root)
}
