package models

import (
	"context"
	"time"

	einoollama "github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/components/model"

	"github.com/dohr-michael/ozzie/internal/config"
)

const defaultOllamaBaseURL = "http://localhost:11434"

// NewOllama creates a new Ollama ChatModel.
func NewOllama(ctx context.Context, cfg config.ProviderConfig) (model.ToolCallingChatModel, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}

	modelConfig := &einoollama.ChatModelConfig{
		BaseURL: baseURL,
		Model:   cfg.Model,
	}

	if cfg.Timeout.Duration() > 0 {
		modelConfig.Timeout = cfg.Timeout.Duration()
	} else {
		modelConfig.Timeout = 300 * time.Second
	}

	opts := &einoollama.Options{}

	if cfg.MaxTokens > 0 {
		opts.NumPredict = cfg.MaxTokens
	}

	if len(cfg.Options) > 0 {
		if temp, ok := cfg.Options["temperature"].(float64); ok {
			opts.Temperature = float32(temp)
		}
		if numCtx, ok := cfg.Options["num_ctx"].(float64); ok {
			opts.NumCtx = int(numCtx)
		}
		if numPredict, ok := cfg.Options["num_predict"].(float64); ok {
			opts.NumPredict = int(numPredict)
		}
		if topP, ok := cfg.Options["top_p"].(float64); ok {
			opts.TopP = float32(topP)
		}
		if topK, ok := cfg.Options["top_k"].(float64); ok {
			opts.TopK = int(topK)
		}
	}

	modelConfig.Options = opts

	return einoollama.NewChatModel(ctx, modelConfig)
}
