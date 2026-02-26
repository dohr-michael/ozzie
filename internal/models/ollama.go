package models

import (
	"context"
	"io"
	"net/http"
	"strings"
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

	// Inject a validating transport to detect non-JSON responses (e.g. "no available server").
	inner := http.DefaultTransport
	modelConfig.HTTPClient = &http.Client{
		Timeout:   modelConfig.Timeout,
		Transport: &ollamaTransport{inner: inner, provider: "ollama"},
	}

	return einoollama.NewChatModel(ctx, modelConfig)
}

// ollamaTransport wraps an http.RoundTripper to detect non-JSON error responses
// from Ollama backends (e.g. reverse proxies returning plain text errors).
type ollamaTransport struct {
	inner    http.RoundTripper
	provider string
}

func (t *ollamaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, &ErrModelUnavailable{Provider: t.provider, Cause: err}
	}

	// Non-2xx: read body and return structured error
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, &ErrModelUnavailable{
			Provider: t.provider,
			Body:     strings.TrimSpace(string(body)),
		}
	}

	// Ollama sends application/x-ndjson for streaming, application/json otherwise.
	// A reverse proxy returning plain text (e.g. "no available server") won't have a JSON content type.
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "json") && !strings.Contains(ct, "ndjson") {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, &ErrModelUnavailable{
			Provider: t.provider,
			Body:     strings.TrimSpace(string(body)),
		}
	}

	return resp, nil
}
