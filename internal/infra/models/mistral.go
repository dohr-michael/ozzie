package models

import (
	"context"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"github.com/dohr-michael/ozzie/internal/config"
)

const (
	defaultMistralBaseURL = "https://api.mistral.ai/v1"
	defaultMistralModel   = "mistral-small-latest"
)

// NewMistral creates a new Mistral AI ChatModel via the OpenAI-compatible API.
func NewMistral(ctx context.Context, cfg config.ProviderConfig, auth ResolvedAuth) (model.ToolCallingChatModel, error) {
	modelName := cfg.Model
	if modelName == "" {
		modelName = defaultMistralModel
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultMistralBaseURL
	}

	modelConfig := &einoopenai.ChatModelConfig{
		APIKey:  auth.Value,
		Model:   modelName,
		BaseURL: baseURL,
	}

	if cfg.MaxTokens > 0 {
		maxTokens := cfg.MaxTokens
		modelConfig.MaxCompletionTokens = &maxTokens
	}

	if cfg.Timeout.Duration() > 0 {
		modelConfig.Timeout = cfg.Timeout.Duration()
	} else {
		modelConfig.Timeout = 5 * time.Minute
	}

	if cfg.Options != nil {
		if temp, ok := cfg.Options["temperature"].(float64); ok {
			t := float32(temp)
			modelConfig.Temperature = &t
		}
		if topP, ok := cfg.Options["top_p"].(float64); ok {
			p := float32(topP)
			modelConfig.TopP = &p
		}
	}

	return einoopenai.NewChatModel(ctx, modelConfig)
}
