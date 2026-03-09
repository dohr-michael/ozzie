package layeredctx

import (
	"testing"
)

func TestBM25Build(t *testing.T) {
	bm := NewBM25(1.2, 0.75)
	docs := []string{
		"the quick brown fox jumps over the lazy dog",
		"a fast red car drives on the highway",
		"the brown dog sleeps in the garden",
	}
	bm.Build(docs)

	if bm.numDocs != 3 {
		t.Errorf("numDocs = %d, want 3", bm.numDocs)
	}
	if bm.avgDL == 0 {
		t.Error("avgDL should be > 0")
	}
	if len(bm.idf) == 0 {
		t.Error("idf should have entries")
	}
}

func TestBM25ScoreRelevant(t *testing.T) {
	bm := NewBM25(1.2, 0.75)
	docs := []string{
		"golang concurrency patterns with goroutines and channels",
		"french cooking recipes for beginners",
		"machine learning with python and tensorflow",
	}
	bm.Build(docs)

	score0 := bm.Score("golang goroutines", docs[0])
	score1 := bm.Score("golang goroutines", docs[1])
	score2 := bm.Score("golang goroutines", docs[2])

	if score0 <= score1 {
		t.Errorf("relevant doc should score higher: score0=%f, score1=%f", score0, score1)
	}
	if score0 <= score2 {
		t.Errorf("relevant doc should score higher: score0=%f, score2=%f", score0, score2)
	}
}

func TestBM25ScoreAllLength(t *testing.T) {
	bm := NewBM25(1.2, 0.75)
	docs := []string{"hello world", "goodbye world", "hello there"}
	bm.Build(docs)

	scores := bm.ScoreAll("hello", docs)
	if len(scores) != 3 {
		t.Fatalf("ScoreAll returned %d scores, want 3", len(scores))
	}
}

func TestBM25PhraseBonus(t *testing.T) {
	bm := NewBM25(1.2, 0.75)
	docs := []string{
		"the context window management system",
		"context management and window sizing",
	}
	bm.Build(docs)

	// Doc 0 contains the exact phrase "context window"
	score0 := bm.Score("context window", docs[0])
	score1 := bm.Score("context window", docs[1])

	if score0 <= score1 {
		t.Errorf("phrase match should score higher: score0=%f, score1=%f", score0, score1)
	}
}

func TestBM25EmptyCorpus(t *testing.T) {
	bm := NewBM25(1.2, 0.75)
	bm.Build(nil)

	score := bm.Score("anything", "some doc")
	if score != 0 {
		t.Errorf("empty corpus should give score 0, got %f", score)
	}
}
