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

	// Configure thinking for Gemini 2.5+ models.
	// Default: enable thinking with budget = 80% of max_tokens to leave room for the response.
	// Can be overridden via options.thinking_budget (0 = disabled, -1 = model default, N = token budget).
	thinkingBudget := int32(-1) // sentinel: auto-configure
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
		if tb, ok := cfg.Options["thinking_budget"].(float64); ok {
			thinkingBudget = int32(tb)
		}
	}

	if thinkingBudget == -1 {
		// Auto: set a reasonable budget so thinking doesn't consume all output tokens.
		budget := int32(8192)
		modelConfig.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingBudget: &budget,
		}
	} else if thinkingBudget == 0 {
		// Explicitly disabled.
		zero := int32(0)
		modelConfig.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingBudget: &zero,
		}
	} else if thinkingBudget > 0 {
		modelConfig.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingBudget: &thinkingBudget,
		}
	}

	return einogemini.NewChatModel(ctx, modelConfig)
}
