package models

import (
	"context"
	"net/http"
	"time"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"

	"github.com/dohr-michael/ozzie/internal/config"
)

const (
	defaultAnthropicModel     = "claude-sonnet-4-6"
	defaultAnthropicMaxTokens = 4096
)

// NewAnthropic creates a new Anthropic ToolCallingChatModel via the eino-ext Claude driver.
func NewAnthropic(ctx context.Context, cfg config.ProviderConfig, auth ResolvedAuth) (model.ToolCallingChatModel, error) {
	modelName := cfg.Model
	if modelName == "" {
		modelName = defaultAnthropicModel
	}
	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultAnthropicMaxTokens
	}

	timeout := 5 * time.Minute
	if cfg.Timeout.Duration() > 0 {
		timeout = cfg.Timeout.Duration()
	}

	modelConfig := &claude.Config{
		Model:      modelName,
		MaxTokens:  maxTokens,
		HTTPClient: &http.Client{Timeout: timeout},
	}

	switch auth.Kind {
	case AuthBearerToken:
		modelConfig.HTTPClient.Transport = &bearerAuthTransport{token: auth.Value}
	default:
		modelConfig.APIKey = auth.Value
	}

	if cfg.BaseURL != "" {
		modelConfig.BaseURL = &cfg.BaseURL
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
		if tb, ok := cfg.Options["thinking_budget"].(float64); ok && tb > 0 {
			modelConfig.Thinking = &claude.Thinking{
				Enable:       true,
				BudgetTokens: int(tb),
			}
		}
	}

	return claude.NewChatModel(ctx, modelConfig)
}

// bearerAuthTransport replaces the X-Api-Key header (set by the eino-ext module)
// with an Authorization: Bearer header for OAuth/token-based auth.
type bearerAuthTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Del("X-Api-Key")
	req.Header.Set("Authorization", "Bearer "+t.token)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
