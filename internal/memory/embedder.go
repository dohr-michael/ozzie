package memory

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/embedding"

	einoollama "github.com/cloudwego/eino-ext/components/embedding/ollama"
	einoopenai "github.com/cloudwego/eino-ext/components/embedding/openai"

	"github.com/dohr-michael/ozzie/internal/config"
)

// NewEmbedder creates an Eino Embedder from the embedding config.
// Supported drivers: "openai", "ollama".
func NewEmbedder(ctx context.Context, cfg config.EmbeddingConfig) (embedding.Embedder, error) {
	switch strings.ToLower(cfg.Driver) {
	case "openai":
		return newOpenAIEmbedder(ctx, cfg)
	case "ollama":
		return newOllamaEmbedder(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported embedding driver %q (supported: openai, ollama)", cfg.Driver)
	}
}

func newOpenAIEmbedder(ctx context.Context, cfg config.EmbeddingConfig) (embedding.Embedder, error) {
	apiKey := resolveEmbeddingAuth(cfg)
	if apiKey == "" {
		return nil, fmt.Errorf("openai embedding: API key not configured (set auth.api_key or OPENAI_API_KEY)")
	}

	ecfg := &einoopenai.EmbeddingConfig{
		APIKey: apiKey,
		Model:  cfg.Model,
	}
	if cfg.BaseURL != "" {
		ecfg.BaseURL = cfg.BaseURL
	}
	if cfg.Dims > 0 {
		dims := cfg.Dims
		ecfg.Dimensions = &dims
	}
	return einoopenai.NewEmbedder(ctx, ecfg)
}

func newOllamaEmbedder(ctx context.Context, cfg config.EmbeddingConfig) (embedding.Embedder, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	ecfg := &einoollama.EmbeddingConfig{
		BaseURL: baseURL,
		Model:   cfg.Model,
	}
	return einoollama.NewEmbedder(ctx, ecfg)
}

// resolveEmbeddingAuth resolves the API key for the embedding provider.
// Resolution order: direct api_key → OPENAI_API_KEY env.
func resolveEmbeddingAuth(cfg config.EmbeddingConfig) string {
	key := strings.TrimSpace(cfg.Auth.APIKey)
	if key != "" {
		if strings.HasPrefix(key, "${") && strings.HasSuffix(key, "}") {
			return os.Getenv(key[2 : len(key)-1])
		}
		return key
	}
	// Default env var per driver
	switch strings.ToLower(cfg.Driver) {
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	default:
		return ""
	}
}
