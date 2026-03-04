package wizard

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/secrets"
)

const defaultDotenv = `# Ozzie environment variables
# This file is loaded automatically. Existing env vars are never overridden.

# ANTHROPIC_API_KEY=sk-ant-...
# OPENAI_API_KEY=sk-...
`

// Finalize creates directories, generates age keys, writes config and .env.
func Finalize(answers Answers) (string, error) {
	root := config.OzziePath()

	// 1. Create directories.
	dirs := []string{
		root,
		filepath.Join(root, "logs"),
		filepath.Join(root, "skills"),
		filepath.Join(root, "sessions"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return "", fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	// 2. Migrate legacy .age-key → .age/current.key (idempotent).
	if err := secrets.MigrateToAgeDir(root); err != nil {
		return "", fmt.Errorf("migrate age key: %w", err)
	}

	// 3. Generate age key (idempotent).
	ageDir := filepath.Join(root, ".age")
	if err := os.MkdirAll(ageDir, 0o700); err != nil {
		return "", fmt.Errorf("create .age dir: %w", err)
	}
	currentKeyPath := filepath.Join(ageDir, "current.key")
	if err := secrets.GenerateIdentity(currentKeyPath); err != nil {
		return "", fmt.Errorf("generate age identity: %w", err)
	}

	// 4. Build and write config.jsonc.
	data := BuildConfigData(answers)
	configContent, err := RenderConfig(data)
	if err != nil {
		return "", fmt.Errorf("render config: %w", err)
	}
	configPath := config.ConfigPath()
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}

	// 5. Encrypt and write API key if provided.
	dotenvPath := config.DotenvPath()
	apiKey := answers.String("api_key", "")
	envVarName := answers.String("env_var_name", "")

	if apiKey != "" && envVarName != "" {
		kr, err := secrets.NewKeyRing()
		if err != nil {
			return "", fmt.Errorf("load keyring: %w", err)
		}
		if kr == nil {
			return "", fmt.Errorf("no keyring available after key generation")
		}

		encrypted, err := secrets.Encrypt(apiKey, kr.CurrentRecipient())
		if err != nil {
			return "", fmt.Errorf("encrypt api key: %w", err)
		}

		if err := secrets.SetEntry(dotenvPath, envVarName, encrypted); err != nil {
			return "", fmt.Errorf("write api key to .env: %w", err)
		}
	} else {
		// Write default .env if missing.
		if _, err := os.Stat(dotenvPath); os.IsNotExist(err) {
			if err := os.WriteFile(dotenvPath, []byte(defaultDotenv), 0o600); err != nil {
				return "", fmt.Errorf("write .env: %w", err)
			}
		}
	}

	return formatSuccessMessage(root, data, apiKey != ""), nil
}

func formatSuccessMessage(root string, data ConfigData, hasKey bool) string {
	msg := fmt.Sprintf(`
  Ozzie is ready.

  Home:     %s
  Provider: %s (%s)
  Gateway:  %s:%d
`, root, data.Driver, data.Model, data.GatewayHost, data.GatewayPort)

	if hasKey {
		msg += fmt.Sprintf("  API Key:  encrypted in .env (%s)\n", data.AuthEnvVar)
	} else if data.AuthEnvVar != "" {
		msg += fmt.Sprintf(`
  Next: set your API key
    ozzie secret set %s
`, data.AuthEnvVar)
	}

	msg += "\n  Run: ozzie gateway\n"
	return msg
}
