package memory

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"

	"github.com/cloudwego/eino/components/embedding"
)

// VectorResult holds a single vector search result.
type VectorResult struct {
	ID         string
	Content    string
	Similarity float32
	Metadata   map[string]string
}

// SQLiteVectorStore stores embeddings as BLOBs in a standard SQLite table
// and performs brute-force cosine similarity search in Go.
type SQLiteVectorStore struct {
	db       *sql.DB
	embedder embedding.Embedder
	dims     int // embedding dimensions
}

// NewSQLiteVectorStore creates or opens the embeddings table.
// embedder is used to compute embeddings for upsert and query.
// dims must match the embedding model's output dimension.
func NewSQLiteVectorStore(db *sql.DB, embedder embedding.Embedder, dims int) (*SQLiteVectorStore, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS memory_embeddings (
		id TEXT PRIMARY KEY,
		embedding BLOB NOT NULL
	)`)
	if err != nil {
		return nil, fmt.Errorf("create embeddings table: %w", err)
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
	blob := encodeEmbedding(vec)

	_, err = vs.db.Exec(`INSERT OR REPLACE INTO memory_embeddings(id, embedding) VALUES (?, ?)`,
		id, blob)
	if err != nil {
		return fmt.Errorf("upsert vector: %w", err)
	}
	return nil
}

// Delete removes a document's embedding.
func (vs *SQLiteVectorStore) Delete(_ context.Context, id string) error {
	_, err := vs.db.Exec(`DELETE FROM memory_embeddings WHERE id = ?`, id)
	return err
}

// Query performs a brute-force cosine similarity search and returns the top results.
func (vs *SQLiteVectorStore) Query(ctx context.Context, queryText string, nResults int) ([]VectorResult, error) {
	queryVec, err := vs.embed(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("embed for query: %w", err)
	}

	rows, err := vs.db.Query(`SELECT id, embedding FROM memory_embeddings`)
	if err != nil {
		return nil, fmt.Errorf("load embeddings: %w", err)
	}
	defer rows.Close()

	var results []VectorResult
	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, fmt.Errorf("scan embedding: %w", err)
		}
		vec := decodeEmbedding(blob)
		sim := cosineSimilarity(queryVec, vec)
		results = append(results, VectorResult{
			ID:         id,
			Similarity: sim,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embeddings: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	if len(results) > nResults {
		results = results[:nResults]
	}
	return results, nil
}

// Count returns the number of vectors stored.
func (vs *SQLiteVectorStore) Count() int {
	var count int
	_ = vs.db.QueryRow(`SELECT count(*) FROM memory_embeddings`).Scan(&count)
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

// encodeEmbedding converts float32 slice to little-endian bytes.
func encodeEmbedding(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// decodeEmbedding converts little-endian bytes back to float32 slice.
func decodeEmbedding(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

// cosineSimilarity computes cosine similarity between two vectors.
// Since embed() normalizes to unit vectors, this is just a dot product.
func cosineSimilarity(a, b []float32) float32 {
	var dot float32
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot
}
