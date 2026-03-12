package setup_wizard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/i18n"
	"github.com/dohr-michael/ozzie/internal/infra/secrets"
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

	// 5. Encrypt and write API keys for each provider.
	dotenvPath := config.DotenvPath()
	providers := answers.Providers()
	hasAnyKey := false

	for _, p := range providers {
		if p.APIKey == "" || p.EnvVarName == "" {
			continue
		}
		hasAnyKey = true

		kr, err := secrets.NewKeyRing()
		if err != nil {
			return "", fmt.Errorf("load keyring: %w", err)
		}
		if kr == nil {
			return "", fmt.Errorf("no keyring available after key generation")
		}

		encrypted, err := secrets.Encrypt(p.APIKey, kr.CurrentRecipient())
		if err != nil {
			return "", fmt.Errorf("encrypt api key for %s: %w", p.Alias, err)
		}

		if err := secrets.SetEntry(dotenvPath, p.EnvVarName, encrypted); err != nil {
			return "", fmt.Errorf("write api key to .env for %s: %w", p.Alias, err)
		}
	}

	// Encrypt embedding API key if present.
	if emb := answers.Embedding(); emb != nil && emb.APIKey != "" && emb.EnvVarName != "" {
		hasAnyKey = true

		kr, err := secrets.NewKeyRing()
		if err != nil {
			return "", fmt.Errorf("load keyring: %w", err)
		}
		if kr == nil {
			return "", fmt.Errorf("no keyring available after key generation")
		}

		encrypted, err := secrets.Encrypt(emb.APIKey, kr.CurrentRecipient())
		if err != nil {
			return "", fmt.Errorf("encrypt embedding api key: %w", err)
		}

		if err := secrets.SetEntry(dotenvPath, emb.EnvVarName, encrypted); err != nil {
			return "", fmt.Errorf("write embedding api key to .env: %w", err)
		}
	}

	// Encrypt MCP server env vars that are secrets.
	for _, srv := range answers.MCPServers() {
		for _, ev := range srv.EnvVars {
			if !ev.IsSecret || ev.Value == "" {
				continue
			}
			hasAnyKey = true

			kr, err := secrets.NewKeyRing()
			if err != nil {
				return "", fmt.Errorf("load keyring: %w", err)
			}
			if kr == nil {
				return "", fmt.Errorf("no keyring available after key generation")
			}

			encrypted, err := secrets.Encrypt(ev.Value, kr.CurrentRecipient())
			if err != nil {
				return "", fmt.Errorf("encrypt mcp env %s for %s: %w", ev.Name, srv.Name, err)
			}

			if err := secrets.SetEntry(dotenvPath, ev.Name, encrypted); err != nil {
				return "", fmt.Errorf("write mcp env %s to .env for %s: %w", ev.Name, srv.Name, err)
			}
		}
	}

	// Write default .env if missing and no keys were written.
	if !hasAnyKey {
		if _, err := os.Stat(dotenvPath); os.IsNotExist(err) {
			if err := os.WriteFile(dotenvPath, []byte(defaultDotenv), 0o600); err != nil {
				return "", fmt.Errorf("write .env: %w", err)
			}
		}
	}

	return formatSuccessMessage(root, data, providers, answers.Embedding(), answers.LayeredContext(), answers.MCPServers()), nil
}

func formatSuccessMessage(root string, data ConfigData, providers []ProviderEntry, emb *EmbeddingEntry, lc *LayeredContextEntry, mcpServers []MCPServerEntry) string {
	var b strings.Builder

	b.WriteString(i18n.T("wizard.final.ready"))
	b.WriteString(fmt.Sprintf(i18n.T("wizard.final.home"), root))
	b.WriteString(fmt.Sprintf(i18n.T("wizard.final.gateway"), data.GatewayHost, data.GatewayPort))
	b.WriteString(fmt.Sprintf(i18n.T("wizard.final.default"), data.DefaultProvider))

	// Provider summary.
	for _, p := range providers {
		label := fmt.Sprintf("  %s: %s (%s)", p.Alias, p.Driver, p.Model)
		b.WriteString(label)
		if p.APIKey != "" {
			b.WriteString(i18n.T("wizard.final.key_encrypted"))
		} else if p.EnvVarName != "" && p.SkipKey {
			b.WriteString(fmt.Sprintf(i18n.T("wizard.final.key_later"), p.EnvVarName))
		}
		b.WriteString("\n")
	}

	// Embedding summary.
	if emb != nil && emb.Enabled {
		b.WriteString(fmt.Sprintf(i18n.T("wizard.final.embedding"), emb.Driver, emb.Model, emb.Dims))
	} else {
		b.WriteString(i18n.T("wizard.final.emb_disabled"))
	}

	// Layered context summary.
	if lc != nil && lc.Enabled {
		b.WriteString(fmt.Sprintf(i18n.T("wizard.final.layered"), lc.MaxRecentMessages, lc.MaxArchives))
	} else {
		b.WriteString(i18n.T("wizard.final.layered_disabled"))
	}

	// MCP servers summary.
	if len(mcpServers) > 0 {
		b.WriteString(fmt.Sprintf(i18n.T("wizard.final.mcp"), len(mcpServers)))
		for _, srv := range mcpServers {
			detail := srv.Transport
			if srv.Transport == "stdio" {
				detail += fmt.Sprintf(" (%s)", srv.Command)
			} else {
				detail += fmt.Sprintf(" (%s)", srv.URL)
			}
			b.WriteString(fmt.Sprintf("  %s: %s\n", srv.Name, detail))
		}
	} else {
		b.WriteString(i18n.T("wizard.final.mcp_none"))
	}

	b.WriteString(i18n.T("wizard.final.run"))
	return b.String()
}
