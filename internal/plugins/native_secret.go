package plugins

import (
	"context"
	"encoding/json"
	"fmt"

	"filippo.io/age"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/secrets"
)

// SetSecretTool decrypts an ENC[age:...] blob and writes the plaintext to .env.
type SetSecretTool struct {
	identity *age.X25519Identity
	reloader *config.Reloader
}

// NewSetSecretTool creates a new set_secret tool.
func NewSetSecretTool(identity *age.X25519Identity, reloader *config.Reloader) *SetSecretTool {
	return &SetSecretTool{identity: identity, reloader: reloader}
}

// SetSecretManifest returns the plugin manifest for the set_secret tool.
func SetSecretManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "set_secret",
		Description: "Store an encrypted secret in .env",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "set_secret",
				Description: "Decrypt an ENC[age:...] encrypted value and store it as an environment variable in .env. The value MUST be encrypted (plaintext is rejected). After writing, the config is hot-reloaded so the secret is immediately available.",
				Parameters: map[string]ParamSpec{
					"name": {
						Type:        "string",
						Description: "Environment variable name (e.g. DISCORD_TOKEN)",
						Required:    true,
					},
					"value": {
						Type:        "string",
						Description: "Encrypted value (must be ENC[age:...] format)",
						Required:    true,
					},
				},
			},
		},
	}
}

type setSecretInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (t *SetSecretTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&SetSecretManifest().Tools[0]), nil
}

func (t *SetSecretTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input setSecretInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("set_secret: parse input: %w", err)
	}
	if input.Name == "" {
		return "", fmt.Errorf("set_secret: name is required")
	}
	if input.Value == "" {
		return "", fmt.Errorf("set_secret: value is required")
	}

	// Security guard: reject plaintext values
	if !secrets.IsEncrypted(input.Value) {
		return "", fmt.Errorf("set_secret: value must be encrypted (ENC[age:...] format)")
	}

	// Decrypt
	plaintext, err := secrets.Decrypt(input.Value, t.identity)
	if err != nil {
		return "", fmt.Errorf("set_secret: decrypt: %w", err)
	}

	// Write to .env
	if err := secrets.SetEntry(config.DotenvPath(), input.Name, plaintext); err != nil {
		return "", fmt.Errorf("set_secret: write .env: %w", err)
	}

	// Hot reload config
	if err := t.reloader.Reload(); err != nil {
		return "", fmt.Errorf("set_secret: reload config: %w", err)
	}

	result, _ := json.Marshal(map[string]string{
		"name":   input.Name,
		"status": "stored",
	})
	return string(result), nil
}

var _ tool.InvokableTool = (*SetSecretTool)(nil)
