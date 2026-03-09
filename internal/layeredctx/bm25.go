package layeredctx

import (
	"math"
	"strings"
)

// BM25 implements the Okapi BM25 ranking function.
type BM25 struct {
	k1      float64
	b       float64
	avgDL   float64
	idf     map[string]float64
	numDocs int
}

// NewBM25 creates a BM25 engine with the given parameters.
func NewBM25(k1, b float64) *BM25 {
	return &BM25{
		k1:  k1,
		b:   b,
		idf: make(map[string]float64),
	}
}

// Build computes IDF values and average document length from a corpus.
func (bm *BM25) Build(docs []string) {
	bm.numDocs = len(docs)
	if bm.numDocs == 0 {
		return
	}

	// Count documents containing each term
	df := make(map[string]int)
	totalLen := 0
	for _, doc := range docs {
		tokens := tokenize(doc)
		totalLen += len(tokens)
		seen := make(map[string]bool)
		for _, t := range tokens {
			if !seen[t] {
				df[t]++
				seen[t] = true
			}
		}
	}

	bm.avgDL = float64(totalLen) / float64(bm.numDocs)

	// Compute IDF: log((N - n + 0.5) / (n + 0.5) + 1)
	bm.idf = make(map[string]float64, len(df))
	for term, n := range df {
		bm.idf[term] = math.Log((float64(bm.numDocs)-float64(n)+0.5)/(float64(n)+0.5) + 1.0)
	}
}

// Score returns the BM25 score for a query against a single document.
// The score is normalized to [0, 1) using 1 - exp(-raw).
// A phrase bonus of +0.15 is added if the full query appears in the doc.
func (bm *BM25) Score(query, doc string) float64 {
	if bm.numDocs == 0 {
		return 0
	}

	queryTokens := tokenize(query)
	docTokens := tokenize(doc)
	docLen := float64(len(docTokens))

	// Count term frequency in document
	tf := make(map[string]int)
	for _, t := range docTokens {
		tf[t]++
	}

	raw := 0.0
	for _, qt := range queryTokens {
		idf := bm.idf[qt]
		if idf == 0 {
			continue
		}
		freq := float64(tf[qt])
		numerator := freq * (bm.k1 + 1)
		denominator := freq + bm.k1*(1-bm.b+bm.b*docLen/bm.avgDL)
		raw += idf * numerator / denominator
	}

	// Normalize to [0, 1)
	score := 1 - math.Exp(-raw)

	// Phrase bonus: if the full query string appears verbatim in the doc
	if len(queryTokens) > 1 && strings.Contains(strings.ToLower(doc), strings.ToLower(query)) {
		score += 0.15
		if score > 1 {
			score = 1
		}
	}

	return score
}

// ScoreAll returns BM25 scores for a query against each document.
func (bm *BM25) ScoreAll(query string, docs []string) []float64 {
	scores := make([]float64, len(docs))
	for i, doc := range docs {
		scores[i] = bm.Score(query, doc)
	}
	return scores
}
