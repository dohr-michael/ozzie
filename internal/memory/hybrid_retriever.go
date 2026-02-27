package memory

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

const (
	keywordWeight     = 0.3
	semanticWeight    = 0.7
	minRetrievalScore = 0.25
)

// HybridRetriever combines keyword and semantic search for memory retrieval.
// If no vector store is available, it falls back to keyword-only retrieval.
type HybridRetriever struct {
	store   Store
	keyword *Retriever
	vector  *VectorStore
	mu      sync.Mutex     // serializes reinforcement updates
	wg      sync.WaitGroup // tracks in-flight reinforcement goroutines
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewHybridRetriever creates a hybrid retriever.
// vector may be nil, in which case only keyword retrieval is used.
func NewHybridRetriever(store Store, vector *VectorStore) *HybridRetriever {
	ctx, cancel := context.WithCancel(context.Background())
	return &HybridRetriever{
		store:   store,
		keyword: NewRetriever(store),
		vector:  vector,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Close cancels pending reinforcement goroutines and waits for completion.
func (hr *HybridRetriever) Close() {
	hr.cancel()
	hr.wg.Wait()
}

// Wait blocks until all pending reinforcement goroutines complete.
func (hr *HybridRetriever) Wait() {
	hr.wg.Wait()
}

// Retrieve finds the most relevant memories using hybrid scoring.
// It satisfies the agent.MemoryRetriever interface.
func (hr *HybridRetriever) Retrieve(query string, tags []string, limit int) ([]RetrievedMemory, error) {
	if limit <= 0 {
		limit = 5
	}

	// Keyword-only fallback when vector store is not available
	if hr.vector == nil {
		results, err := hr.keyword.Retrieve(query, tags, limit)
		if err != nil {
			return nil, err
		}
		results = filterByThreshold(results)
		if len(results) > 0 {
			hr.wg.Add(1)
			go hr.reinforceResults(results)
		}
		return results, nil
	}

	// Fetch expanded result sets from both sources
	fetchLimit := limit * 2

	keywordResults, err := hr.keyword.Retrieve(query, tags, fetchLimit)
	if err != nil {
		return nil, err
	}

	semanticResults, err := hr.vector.Query(context.Background(), query, fetchLimit)
	if err != nil {
		// Graceful degradation: reuse keyword results already fetched
		results := keywordResults
		if len(results) > limit {
			results = results[:limit]
		}
		results = filterByThreshold(results)
		if len(results) > 0 {
			hr.wg.Add(1)
			go hr.reinforceResults(results)
		}
		return results, nil
	}

	merged := hr.mergeResults(keywordResults, semanticResults, limit)
	merged = filterByThreshold(merged)
	if len(merged) > 0 {
		hr.wg.Add(1)
		go hr.reinforceResults(merged)
	}
	return merged, nil
}

// mergeResults combines keyword and semantic results with hybrid scoring.
func (hr *HybridRetriever) mergeResults(keywordResults []RetrievedMemory, semanticResults []VectorResult, limit int) []RetrievedMemory {
	type scored struct {
		keywordScore  float64
		semanticScore float64
	}

	merged := make(map[string]*scored)

	// Normalize keyword scores to [0,1]
	var maxKeyword float64
	for _, r := range keywordResults {
		if r.Score > maxKeyword {
			maxKeyword = r.Score
		}
	}
	for _, r := range keywordResults {
		norm := 0.0
		if maxKeyword > 0 {
			norm = r.Score / maxKeyword
		}
		merged[r.Entry.ID] = &scored{keywordScore: norm}
	}

	// Semantic scores from chromem-go are already cosine similarity in [-1,1]
	// Normalize to [0,1] range
	for _, r := range semanticResults {
		sim := float64(r.Similarity+1) / 2 // map [-1,1] → [0,1]
		if s, ok := merged[r.ID]; ok {
			s.semanticScore = sim
		} else {
			merged[r.ID] = &scored{semanticScore: sim}
		}
	}

	// Build result set with hybrid scores
	type hybridResult struct {
		id    string
		score float64
	}
	var results []hybridResult
	for id, s := range merged {
		hybrid := keywordWeight*s.keywordScore + semanticWeight*s.semanticScore
		results = append(results, hybridResult{id: id, score: hybrid})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	// Load full entries for the top results
	var out []RetrievedMemory
	for _, r := range results {
		entry, content, err := hr.store.Get(r.id)
		if err != nil {
			continue
		}
		out = append(out, RetrievedMemory{
			Entry:   entry,
			Content: content,
			Score:   r.score,
		})
	}
	return out
}

// reinforceResults updates LastUsedAt and Confidence for retrieved memories.
// Applies decay before reinforcement so stale memories lose confidence over time.
// Fire-and-forget: errors are logged but do not affect retrieval.
func (hr *HybridRetriever) reinforceResults(results []RetrievedMemory) {
	defer hr.wg.Done()
	now := time.Now()
	for _, r := range results {
		if hr.ctx.Err() != nil {
			return
		}
		hr.mu.Lock()
		entry, content, err := hr.store.Get(r.Entry.ID)
		if err != nil || entry == nil {
			hr.mu.Unlock()
			continue
		}
		// Apply decay before reinforcement
		entry.Confidence = ApplyDecay(entry.Confidence, entry.LastUsedAt, now)
		// Then reinforce
		entry.Confidence = min(entry.Confidence+0.05, 1.0)
		entry.LastUsedAt = now
		if err := hr.store.Update(entry, content); err != nil {
			slog.Debug("reinforce memory failed", "id", r.Entry.ID, "error", err)
		}
		hr.mu.Unlock()
	}
}

// filterByThreshold removes results below the minimum retrieval score.
func filterByThreshold(results []RetrievedMemory) []RetrievedMemory {
	var filtered []RetrievedMemory
	for _, r := range results {
		if r.Score >= minRetrievalScore {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
