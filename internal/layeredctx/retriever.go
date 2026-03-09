package layeredctx

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Retriever selects relevant context from the index using BM25 scoring.
type Retriever struct {
	store *Store
	cfg   Config
}

// NewRetriever creates a Retriever.
func NewRetriever(store *Store, cfg Config) *Retriever {
	return &Retriever{store: store, cfg: cfg}
}

type scoredNode struct {
	node  Node
	score float64
}

// Retrieve selects the most relevant archived context for the given query.
func (r *Retriever) Retrieve(sessionID string, index *Index, query string) (*RetrievalResult, error) {
	if index == nil || len(index.Nodes) == 0 {
		return &RetrievalResult{}, nil
	}

	// Budget: 45% of max prompt tokens, minimum 400
	budget := int(math.Floor(float64(r.cfg.MaxPromptTokens) * 0.45))
	if budget < 400 {
		budget = 400
	}

	// Build BM25 engine from all node abstracts + keywords
	docs := make([]string, len(index.Nodes))
	for i, n := range index.Nodes {
		docs[i] = n.Abstract + " " + strings.Join(n.Keywords, " ")
	}
	bm := NewBM25(1.2, 0.75)
	bm.Build(docs)

	// L0 scoring: abstracts + keywords + recency prior
	scored := make([]scoredNode, len(index.Nodes))
	maxRecency := 0
	for _, n := range index.Nodes {
		if n.Metadata.RecencyRank > maxRecency {
			maxRecency = n.Metadata.RecencyRank
		}
	}

	for i, n := range index.Nodes {
		s := bm.Score(query, docs[i])
		// Recency prior: max +0.08 for the most recent
		if maxRecency > 0 {
			recencyBonus := 0.08 * float64(n.Metadata.RecencyRank) / float64(maxRecency)
			s += recencyBonus
		}
		scored[i] = scoredNode{node: n, score: s}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Check L0 confidence
	topScore := scored[0].score
	decision := RetrievalDecision{TopScore: topScore}

	if topScore >= r.cfg.ScoreThresholdHigh {
		// High confidence at L0 — return top 3 abstracts
		decision.ReachedLayer = LayerL0
		decision.Reason = "high confidence at L0"
		selections := r.selectWithBudget(scored, budget, LayerL0, 3)
		return r.buildResult(selections, decision, budget), nil
	}

	// L1 escalation: re-score using summaries
	topN := r.cfg.MaxItemsL1
	if topN > len(scored) {
		topN = len(scored)
	}
	candidates := scored[:topN]

	summaryDocs := make([]string, len(candidates))
	for i, sn := range candidates {
		summaryDocs[i] = sn.node.Summary + " " + strings.Join(sn.node.Keywords, " ")
	}
	bmL1 := NewBM25(1.2, 0.75)
	bmL1.Build(summaryDocs)

	for i := range candidates {
		candidates[i].score = bmL1.Score(query, summaryDocs[i])
		if maxRecency > 0 {
			recencyBonus := 0.08 * float64(candidates[i].node.Metadata.RecencyRank) / float64(maxRecency)
			candidates[i].score += recencyBonus
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	topScoreL1 := candidates[0].score

	// Check margin between top-1 and top-2
	margin := 0.0
	if len(candidates) > 1 {
		margin = candidates[0].score - candidates[1].score
	}

	if topScoreL1 >= r.cfg.ScoreThresholdHigh || margin >= r.cfg.Top1Top2Margin {
		decision.ReachedLayer = LayerL1
		decision.Reason = fmt.Sprintf("L1 confidence: score=%.3f margin=%.3f", topScoreL1, margin)
		selections := r.selectWithBudget(candidates, budget, LayerL1, r.cfg.MaxItemsL1)
		return r.buildResult(selections, decision, budget), nil
	}

	// L2 escalation: load full transcripts
	decision.ReachedLayer = LayerL2
	decision.Reason = "escalated to L2 full transcripts"

	topL2 := r.cfg.MaxItemsL2
	if topL2 > len(candidates) {
		topL2 = len(candidates)
	}

	var selections []Selection
	usedTokens := 0

	// Always include root abstract first
	rootAbstract := index.Root.Abstract
	rootTokens := EstimateTokens(rootAbstract)
	if rootTokens <= budget {
		selections = append(selections, Selection{
			NodeID:  "root",
			Layer:   LayerL0,
			Content: rootAbstract,
			Tokens:  rootTokens,
			Score:   1.0,
		})
		usedTokens += rootTokens
	}

	for i := 0; i < topL2; i++ {
		archive, err := r.store.ReadArchive(sessionID, candidates[i].node.ID)
		if err != nil || archive == nil {
			// Fallback to summary
			content := candidates[i].node.Summary
			tokens := EstimateTokens(content)
			if usedTokens+tokens <= budget {
				selections = append(selections, Selection{
					NodeID:  candidates[i].node.ID,
					Layer:   LayerL1,
					Content: content,
					Tokens:  tokens,
					Score:   candidates[i].score,
				})
				usedTokens += tokens
			}
			continue
		}

		content := archive.Transcript
		tokens := EstimateTokens(content)
		if usedTokens+tokens <= budget {
			selections = append(selections, Selection{
				NodeID:  candidates[i].node.ID,
				Layer:   LayerL2,
				Content: content,
				Tokens:  tokens,
				Score:   candidates[i].score,
			})
			usedTokens += tokens
		} else {
			// Too large — fall back to summary
			content = candidates[i].node.Summary
			tokens = EstimateTokens(content)
			if usedTokens+tokens <= budget {
				selections = append(selections, Selection{
					NodeID:  candidates[i].node.ID,
					Layer:   LayerL1,
					Content: content,
					Tokens:  tokens,
					Score:   candidates[i].score,
				})
				usedTokens += tokens
			}
		}
	}

	return r.buildResult(selections, decision, budget), nil
}

// selectWithBudget greedily picks items at the given layer until budget exhausted.
func (r *Retriever) selectWithBudget(scored []scoredNode, budget int, layer Layer, maxItems int) []Selection {
	var selections []Selection
	usedTokens := 0

	for i := 0; i < len(scored) && i < maxItems; i++ {
		var content string
		switch layer {
		case LayerL0:
			content = scored[i].node.Abstract
		case LayerL1:
			content = scored[i].node.Summary
		default:
			content = scored[i].node.Summary
		}

		tokens := EstimateTokens(content)
		if usedTokens+tokens > budget {
			continue
		}

		selections = append(selections, Selection{
			NodeID:  scored[i].node.ID,
			Layer:   layer,
			Content: content,
			Tokens:  tokens,
			Score:   scored[i].score,
		})
		usedTokens += tokens
	}
	return selections
}

func (r *Retriever) buildResult(selections []Selection, decision RetrievalDecision, budget int) *RetrievalResult {
	usedTokens := 0
	for _, s := range selections {
		usedTokens += s.Tokens
	}

	savings := 0.0
	if budget > 0 {
		savings = 1.0 - float64(usedTokens)/float64(budget)
	}

	return &RetrievalResult{
		Selections: selections,
		Decision:   decision,
		TokenUsage: TokenUsage{
			Budget:       budget,
			Used:         usedTokens,
			Total:        usedTokens,
			SavingsRatio: savings,
		},
	}
}
