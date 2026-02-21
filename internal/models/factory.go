package models

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"

	"github.com/dohr-michael/ozzie/internal/config"
)

// CreateModel creates a model.ToolCallingChatModel from a provider config.
func CreateModel(ctx context.Context, cfg config.ProviderConfig) (model.ToolCallingChatModel, error) {
	switch strings.ToLower(cfg.Driver) {
	case "anthropic":
		auth, err := ResolveAuth(cfg)
		if err != nil {
			return nil, fmt.Errorf("resolve auth: %w", err)
		}
		return NewAnthropic(ctx, cfg, auth)
	case "openai":
		auth, err := ResolveAuth(cfg)
		if err != nil {
			return nil, fmt.Errorf("resolve auth: %w", err)
		}
		return NewOpenAI(ctx, cfg, auth)
	case "ollama":
		return NewOllama(ctx, cfg)
	default:
		return nil, fmt.Errorf("unknown driver: %s", cfg.Driver)
	}
}
