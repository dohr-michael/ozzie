package memory

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cloudwego/eino/components/embedding"
	chromem "github.com/philippgille/chromem-go"
)

const collectionName = "ozzie_memories"

// VectorResult holds a single vector search result.
type VectorResult struct {
	ID         string
	Content    string
	Similarity float32
	Metadata   map[string]string
}

// VectorStore wraps chromem-go for persistent vector storage.
type VectorStore struct {
	db         *chromem.DB
	collection *chromem.Collection
}

// NewVectorStore creates a persistent vector store in the given directory.
// The embedder is bridged from Eino's [][]float64 to chromem-go's []float32.
func NewVectorStore(ctx context.Context, dir string, embedder embedding.Embedder) (*VectorStore, error) {
	vectorDir := filepath.Join(dir, "vectors")
	db, err := chromem.NewPersistentDB(vectorDir, false)
	if err != nil {
		return nil, fmt.Errorf("open vector store: %w", err)
	}

	ef := bridgeEmbedder(ctx, embedder)
	col, err := db.GetOrCreateCollection(collectionName, nil, ef)
	if err != nil {
		return nil, fmt.Errorf("get or create collection: %w", err)
	}

	return &VectorStore{db: db, collection: col}, nil
}

// Upsert adds or updates a document in the vector store.
func (vs *VectorStore) Upsert(ctx context.Context, id, content string, meta map[string]string) error {
	// chromem-go's Add handles upsert (overwrites existing ID)
	return vs.collection.Add(ctx, []string{id}, nil, []map[string]string{meta}, []string{content})
}

// Delete removes a document from the vector store.
func (vs *VectorStore) Delete(ctx context.Context, id string) error {
	return vs.collection.Delete(ctx, nil, nil, id)
}

// Query performs a semantic search and returns the top results.
func (vs *VectorStore) Query(ctx context.Context, queryText string, nResults int) ([]VectorResult, error) {
	if vs.collection.Count() == 0 {
		return nil, nil
	}
	if nResults > vs.collection.Count() {
		nResults = vs.collection.Count()
	}

	results, err := vs.collection.Query(ctx, queryText, nResults, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("vector query: %w", err)
	}

	out := make([]VectorResult, len(results))
	for i, r := range results {
		out[i] = VectorResult{
			ID:         r.ID,
			Content:    r.Content,
			Similarity: r.Similarity,
			Metadata:   r.Metadata,
		}
	}
	return out, nil
}

// Count returns the number of documents in the vector store.
func (vs *VectorStore) Count() int {
	return vs.collection.Count()
}

// bridgeEmbedder converts an Eino Embedder ([][]float64) to a chromem-go EmbeddingFunc ([]float32).
func bridgeEmbedder(ctx context.Context, embedder embedding.Embedder) chromem.EmbeddingFunc {
	return func(embedCtx context.Context, text string) ([]float32, error) {
		// Use the parent context if the embed context is background
		if embedCtx == context.Background() {
			embedCtx = ctx
		}
		vectors, err := embedder.EmbedStrings(embedCtx, []string{text})
		if err != nil {
			return nil, fmt.Errorf("embed text: %w", err)
		}
		if len(vectors) == 0 || len(vectors[0]) == 0 {
			return nil, fmt.Errorf("embed text: empty result")
		}

		// Convert float64 → float32
		f64 := vectors[0]
		f32 := make([]float32, len(f64))
		for i, v := range f64 {
			f32[i] = float32(v)
		}
		return f32, nil
	}
}
