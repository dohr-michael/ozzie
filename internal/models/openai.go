package models

import (
	"context"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"github.com/dohr-michael/ozzie/internal/config"
)

// NewOpenAI creates a new OpenAI ChatModel.
func NewOpenAI(ctx context.Context, cfg config.ProviderConfig, auth ResolvedAuth) (model.ToolCallingChatModel, error) {
	modelConfig := &einoopenai.ChatModelConfig{
		APIKey: auth.Value,
		Model:  cfg.Model,
	}

	if cfg.BaseURL != "" {
		modelConfig.BaseURL = cfg.BaseURL
	}

	if cfg.MaxTokens > 0 {
		maxTokens := cfg.MaxTokens
		modelConfig.MaxCompletionTokens = &maxTokens
	}

	if cfg.Timeout.Duration() > 0 {
		modelConfig.Timeout = cfg.Timeout.Duration()
	} else {
		modelConfig.Timeout = 60 * time.Second
	}

	if cfg.Options != nil {
		if temp, ok := cfg.Options["temperature"].(float64); ok {
			t := float32(temp)
			modelConfig.Temperature = &t
		}
	}

	return einoopenai.NewChatModel(ctx, modelConfig)
}
