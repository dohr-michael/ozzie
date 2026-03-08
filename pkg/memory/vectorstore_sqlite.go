package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"

	"github.com/cloudwego/eino/components/embedding"
)

// VectorResult holds a single vector search result.
type VectorResult struct {
	ID         string
	Content    string
	Similarity float32
	Metadata   map[string]string
}

// SQLiteVectorStore wraps sqlite-vec for persistent vector storage.
// Embeddings are computed externally (via embedder) and stored in vec0 table.
type SQLiteVectorStore struct {
	db       *sql.DB
	embedder embedding.Embedder
	dims     int // embedding dimensions
}

// NewSQLiteVectorStore creates or opens the vec0 virtual table.
// embedder is used to compute embeddings for upsert and query.
// dims must match the embedding model's output dimension.
func NewSQLiteVectorStore(db *sql.DB, embedder embedding.Embedder, dims int) (*SQLiteVectorStore, error) {
	createSQL := fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS memory_vectors USING vec0(
		id TEXT PRIMARY KEY,
		embedding float[%d]
	)`, dims)
	if _, err := db.Exec(createSQL); err != nil {
		return nil, fmt.Errorf("create vec0 table: %w", err)
	}
	return &SQLiteVectorStore{db: db, embedder: embedder, dims: dims}, nil
}

// Upsert adds or updates a document's embedding.
// content is the text to embed (already formatted by BuildEmbedText).
func (vs *SQLiteVectorStore) Upsert(ctx context.Context, id, content string, _ map[string]string) error {
	vec, err := vs.embed(ctx, content)
	if err != nil {
		return fmt.Errorf("embed for upsert: %w", err)
	}
	vecJSON, _ := json.Marshal(vec)

	_, err = vs.db.Exec(`INSERT OR REPLACE INTO memory_vectors(id, embedding) VALUES (?, ?)`,
		id, string(vecJSON))
	if err != nil {
		return fmt.Errorf("upsert vector: %w", err)
	}
	return nil
}

// Delete removes a document's embedding.
func (vs *SQLiteVectorStore) Delete(_ context.Context, id string) error {
	_, err := vs.db.Exec(`DELETE FROM memory_vectors WHERE id = ?`, id)
	return err
}

// Query performs a semantic search and returns the top results.
func (vs *SQLiteVectorStore) Query(ctx context.Context, queryText string, nResults int) ([]VectorResult, error) {
	vec, err := vs.embed(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("embed for query: %w", err)
	}
	vecJSON, _ := json.Marshal(vec)

	rows, err := vs.db.Query(`SELECT id, distance
		FROM memory_vectors
		WHERE embedding MATCH ?
		ORDER BY distance
		LIMIT ?`, string(vecJSON), nResults)
	if err != nil {
		return nil, fmt.Errorf("vec query: %w", err)
	}
	defer rows.Close()

	var results []VectorResult
	for rows.Next() {
		var id string
		var distance float64
		if err := rows.Scan(&id, &distance); err != nil {
			return nil, fmt.Errorf("scan vec result: %w", err)
		}
		// Convert L2 distance to similarity in [-1, 1] range for compatibility
		// with existing HybridRetriever scoring
		similarity := 1.0 / (1.0 + distance)
		results = append(results, VectorResult{
			ID:         id,
			Similarity: float32(similarity),
		})
	}
	return results, rows.Err()
}

// Count returns the number of vectors stored.
func (vs *SQLiteVectorStore) Count() int {
	var count int
	_ = vs.db.QueryRow(`SELECT count(*) FROM memory_vectors`).Scan(&count)
	return count
}

// embed computes the embedding for a text string.
func (vs *SQLiteVectorStore) embed(ctx context.Context, text string) ([]float32, error) {
	vectors, err := vs.embedder.EmbedStrings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}

	f64 := vectors[0]
	// Normalize to unit vector
	var norm float64
	for _, v := range f64 {
		norm += v * v
	}
	norm = math.Sqrt(norm)

	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		if norm > 0 {
			f32[i] = float32(v / norm)
		} else {
			f32[i] = float32(v)
		}
	}
	return f32, nil
}
