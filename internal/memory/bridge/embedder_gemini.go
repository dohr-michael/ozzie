package bridge

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
	"google.golang.org/genai"

	"github.com/dohr-michael/ozzie/internal/config"
	"github.com/dohr-michael/ozzie/internal/secrets"
)

// geminiEmbedder wraps the Google genai SDK to implement Eino's embedding.Embedder.
type geminiEmbedder struct {
	client *genai.Client
	model  string
}

func newGeminiEmbedder(ctx context.Context, cfg config.EmbeddingConfig, kr *secrets.KeyRing) (embedding.Embedder, error) {
	apiKey := resolveEmbeddingAuth(cfg, kr)
	if apiKey == "" {
		return nil, fmt.Errorf("gemini embedding: API key not configured (set auth.api_key or GOOGLE_API_KEY)")
	}

	clientCfg := &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	}

	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("gemini embedding: create client: %w", err)
	}

	return &geminiEmbedder{
		client: client,
		model:  cfg.Model,
	}, nil
}

// EmbedStrings implements embedding.Embedder.
func (e *geminiEmbedder) EmbedStrings(ctx context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	contents := make([]*genai.Content, len(texts))
	for i, t := range texts {
		contents[i] = &genai.Content{
			Parts: []*genai.Part{genai.NewPartFromText(t)},
		}
	}

	resp, err := e.client.Models.EmbedContent(ctx, e.model, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("gemini embedding: %w", err)
	}

	if len(resp.Embeddings) != len(texts) {
		return nil, fmt.Errorf("gemini embedding: expected %d embeddings, got %d", len(texts), len(resp.Embeddings))
	}

	result := make([][]float64, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		vec := make([]float64, len(emb.Values))
		for j, v := range emb.Values {
			vec[j] = float64(v)
		}
		result[i] = vec
	}
	return result, nil
}
