package models

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/infra/secrets"
)

// CreateModel creates a model.ToolCallingChatModel from a provider config.
// If kr is non-nil, ENC[age:...] auth values are decrypted transparently.
func CreateModel(ctx context.Context, cfg config.ProviderConfig, kr *secrets.KeyRing) (model.ToolCallingChatModel, error) {
	switch strings.ToLower(cfg.Driver) {
	case "anthropic":
		auth, err := ResolveAuth(cfg, kr)
		if err != nil {
			return nil, fmt.Errorf("resolve auth: %w", err)
		}
		return NewAnthropic(ctx, cfg, auth)
	case "openai", "openai-like":
		auth, err := ResolveAuth(cfg, kr)
		if err != nil {
			return nil, fmt.Errorf("resolve auth: %w", err)
		}
		return NewOpenAI(ctx, cfg, auth)
	case "mistral":
		auth, err := ResolveAuth(cfg, kr)
		if err != nil {
			return nil, fmt.Errorf("resolve auth: %w", err)
		}
		return NewMistral(ctx, cfg, auth)
	case "gemini":
		auth, err := ResolveAuth(cfg, kr)
		if err != nil {
			return nil, fmt.Errorf("resolve auth: %w", err)
		}
		return NewGemini(ctx, cfg, auth)
	case "ollama":
		return NewOllama(ctx, cfg)
	default:
		return nil, fmt.Errorf("unknown driver: %s", cfg.Driver)
	}
}
