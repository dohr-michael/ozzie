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
	"github.com/dohr-michael/ozzie/internal/secrets"
)

// NewEmbedder creates an Eino Embedder from the embedding config.
// Supported drivers: "openai", "ollama".
// If kr is non-nil, ENC[age:...] auth values are decrypted transparently.
func NewEmbedder(ctx context.Context, cfg config.EmbeddingConfig, kr *secrets.KeyRing) (embedding.Embedder, error) {
	switch strings.ToLower(cfg.Driver) {
	case "openai":
		return newOpenAIEmbedder(ctx, cfg, kr)
	case "ollama":
		return newOllamaEmbedder(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported embedding driver %q (supported: openai, ollama)", cfg.Driver)
	}
}

func newOpenAIEmbedder(ctx context.Context, cfg config.EmbeddingConfig, kr *secrets.KeyRing) (embedding.Embedder, error) {
	apiKey := resolveEmbeddingAuth(cfg, kr)
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
// If kr is non-nil, ENC[age:...] values are decrypted transparently.
func resolveEmbeddingAuth(cfg config.EmbeddingConfig, kr *secrets.KeyRing) string {
	key := strings.TrimSpace(cfg.Auth.APIKey)
	if key != "" {
		if strings.HasPrefix(key, "${") && strings.HasSuffix(key, "}") {
			return embeddingMaybeDecrypt(os.Getenv(key[2:len(key)-1]), kr)
		}
		return embeddingMaybeDecrypt(key, kr)
	}
	// Default env var per driver
	switch strings.ToLower(cfg.Driver) {
	case "openai":
		return embeddingMaybeDecrypt(os.Getenv("OPENAI_API_KEY"), kr)
	default:
		return ""
	}
}

// embeddingMaybeDecrypt transparently decrypts ENC[age:...] values if a keyring is available.
func embeddingMaybeDecrypt(value string, kr *secrets.KeyRing) string {
	if kr == nil || !secrets.IsEncrypted(value) {
		return value
	}
	if dec, err := kr.DecryptValue(value); err == nil {
		return dec
	}
	return value
}
