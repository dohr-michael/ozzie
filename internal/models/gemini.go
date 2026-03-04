package models

import (
	"context"
	"time"

	einogemini "github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/components/model"
	"google.golang.org/genai"

	"github.com/dohr-michael/ozzie/internal/config"
)

// NewGemini creates a new Gemini ChatModel via the eino-ext driver.
func NewGemini(ctx context.Context, cfg config.ProviderConfig, auth ResolvedAuth) (model.ToolCallingChatModel, error) {
	clientCfg := &genai.ClientConfig{
		APIKey:  auth.Value,
		Backend: genai.BackendGeminiAPI,
	}

	if cfg.BaseURL != "" {
		clientCfg.HTTPOptions.BaseURL = cfg.BaseURL
	}

	if cfg.Timeout.Duration() > 0 {
		d := cfg.Timeout.Duration()
		clientCfg.HTTPOptions.Timeout = &d
	} else {
		d := 5 * time.Minute
		clientCfg.HTTPOptions.Timeout = &d
	}

	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		return nil, err
	}

	modelConfig := &einogemini.Config{
		Client: client,
		Model:  cfg.Model,
	}

	if cfg.MaxTokens > 0 {
		maxTokens := cfg.MaxTokens
		modelConfig.MaxTokens = &maxTokens
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
		if topK, ok := cfg.Options["top_k"].(float64); ok {
			k := int32(topK)
			modelConfig.TopK = &k
		}
	}

	return einogemini.NewChatModel(ctx, modelConfig)
}
