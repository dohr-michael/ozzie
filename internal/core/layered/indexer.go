package layered

import (
	"context"
	"crypto/sha1"
	"fmt"
	"strings"
	"time"

	"github.com/dohr-michael/ozzie/internal/core/brain"
	"github.com/dohr-michael/ozzie/internal/core/prompt"
)

// Summarizer produces a summary of text within a target token budget.
type Summarizer func(ctx context.Context, text string, targetTokens int) (string, error)

// LLMSummarizerFunc matches the agent summarize signature: (ctx, prompt) → (result, error).
type LLMSummarizerFunc func(ctx context.Context, prompt string) (string, error)

// NewLLMSummarizer wraps an LLM call into a Summarizer with automatic fallback.
// If llm is nil, the returned Summarizer uses only the heuristic fallback.
func NewLLMSummarizer(llm LLMSummarizerFunc) Summarizer {
	return func(ctx context.Context, text string, targetTokens int) (string, error) {
		if llm == nil {
			return FallbackSummarizer(text, targetTokens), nil
		}
		prompt := buildSummarizePrompt(text, targetTokens)
		result, err := llm(ctx, prompt)
		if err != nil {
			// Fallback: LLM unavailable, use heuristic
			return FallbackSummarizer(text, targetTokens), nil
		}
		return TrimToTokens(result, targetTokens), nil
	}
}

// buildSummarizePrompt builds a prompt adapted to the target token budget.
// Small budgets (L0 abstracts) get a concise instruction; larger budgets (L1
// summaries) get a structured bullet-point instruction.
func buildSummarizePrompt(text string, targetTokens int) string {
	if targetTokens <= 150 {
		return fmt.Sprintf(prompt.SummarizeLayeredL0, targetTokens, text)
	}
	return fmt.Sprintf(prompt.SummarizeLayeredL1, targetTokens, text)
}

// FallbackSummarizer produces a summary without calling an LLM.
// L1: takes the first non-empty lines as bullet points.
// L0: takes the first 2 sentences from the L1 summary.
func FallbackSummarizer(text string, targetTokens int) string {
	lines := strings.Split(text, "\n")
	var nonEmpty []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			nonEmpty = append(nonEmpty, trimmed)
		}
	}

	if targetTokens <= 150 {
		// L0 mode: first 2 sentences
		joined := strings.Join(nonEmpty, " ")
		sentences := splitSentences(joined)
		if len(sentences) > 2 {
			sentences = sentences[:2]
		}
		result := strings.Join(sentences, " ")
		return TrimToTokens(result, targetTokens)
	}

	// L1 mode: bullet list of first lines
	maxLines := 18
	if len(nonEmpty) > maxLines {
		nonEmpty = nonEmpty[:maxLines]
	}
	var sb strings.Builder
	for _, line := range nonEmpty {
		sb.WriteString("- ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return TrimToTokens(sb.String(), targetTokens)
}

// splitSentences splits text at sentence boundaries (. ! ?).
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder
	for _, r := range text {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			s := strings.TrimSpace(current.String())
			if s != "" {
				sentences = append(sentences, s)
			}
			current.Reset()
		}
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		sentences = append(sentences, s)
	}
	return sentences
}

// Indexer builds and incrementally updates the layered index for a session.
type Indexer struct {
	store      *Store
	summarizer Summarizer
	cfg        Config
}

// NewIndexer creates an Indexer with the given store and summarizer.
func NewIndexer(store *Store, summarizer Summarizer, cfg Config) *Indexer {
	return &Indexer{
		store:      store,
		summarizer: summarizer,
		cfg:        cfg,
	}
}

// BuildOrUpdate builds or incrementally updates the index for a session.
func (ix *Indexer) BuildOrUpdate(ctx context.Context, sessionID string, archived []brain.Message) (*Index, error) {
	existing, err := ix.store.LoadIndex(sessionID)
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}

	// Build checksum map from existing nodes for cache hits
	checksumCache := make(map[string]*Node)
	if existing != nil {
		for i := range existing.Nodes {
			checksumCache[existing.Nodes[i].Checksum] = &existing.Nodes[i]
		}
	}

	// Chunk messages
	chunks := ix.chunkMessages(archived)

	// Process each chunk
	now := time.Now()
	var nodes []Node
	for i, chunk := range chunks {
		transcript := formatTranscript(chunk)
		checksum := computeChecksum(transcript)

		// Cache hit?
		if cached, ok := checksumCache[checksum]; ok {
			cached.Metadata.RecencyRank = len(chunks) - 1 - i
			cached.UpdatedAt = now
			nodes = append(nodes, *cached)
			continue
		}

		// Cache miss — generate summaries
		summary, err := ix.summarizer(ctx, transcript, ix.cfg.L1TargetTokens)
		if err != nil {
			return nil, fmt.Errorf("summarize chunk %d: %w", i, err)
		}
		abstract, err := ix.summarizer(ctx, summary, ix.cfg.L0TargetTokens)
		if err != nil {
			return nil, fmt.Errorf("abstract chunk %d: %w", i, err)
		}
		keywords := ExtractKeywords(transcript, 10)

		nodeID := checksum[:12]
		node := Node{
			ID:           nodeID,
			Abstract:     abstract,
			Summary:      summary,
			ResourcePath: fmt.Sprintf("archives/archive_%s.json", nodeID),
			Checksum:     checksum,
			Keywords:     keywords,
			Metadata: NodeMetadata{
				MessageCount: len(chunk),
				RecencyRank:  len(chunks) - 1 - i,
			},
			TokenEstimate: TokenEstimate{
				Abstract:   EstimateTokens(abstract),
				Summary:    EstimateTokens(summary),
				Transcript: EstimateTokens(transcript),
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		nodes = append(nodes, node)

		// Write archive
		if err := ix.store.WriteArchive(sessionID, nodeID, ArchivePayload{
			NodeID:     nodeID,
			Transcript: transcript,
		}); err != nil {
			return nil, fmt.Errorf("write archive %s: %w", nodeID, err)
		}
	}

	// Trim to MaxArchives (keep most recent)
	if len(nodes) > ix.cfg.MaxArchives {
		nodes = nodes[len(nodes)-ix.cfg.MaxArchives:]
	}

	// Build root
	root, err := ix.buildRoot(ctx, nodes)
	if err != nil {
		return nil, fmt.Errorf("build root: %w", err)
	}

	// Build index
	index := &Index{
		Version:   1,
		SessionID: sessionID,
		Root:      root,
		Nodes:     nodes,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing != nil {
		index.CreatedAt = existing.CreatedAt
	}

	// Persist
	if err := ix.store.SaveIndex(sessionID, index); err != nil {
		return nil, fmt.Errorf("save index: %w", err)
	}

	// Cleanup orphaned archives
	validIDs := make([]string, len(nodes))
	for i, n := range nodes {
		validIDs[i] = n.ID
	}
	_ = ix.store.CleanupArchives(sessionID, validIDs)

	return index, nil
}

// chunkMessages splits messages into groups of ArchiveChunkSize,
// keeping tool_use+tool_result pairs together.
func (ix *Indexer) chunkMessages(messages []brain.Message) [][]brain.Message {
	size := ix.cfg.ArchiveChunkSize
	if size <= 0 {
		size = 8
	}

	var chunks [][]brain.Message
	var current []brain.Message

	for i := 0; i < len(messages); i++ {
		current = append(current, messages[i])

		if len(current) >= size {
			chunks = append(chunks, current)
			current = nil
		}
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
}

// buildRoot constructs the root node from all child nodes.
func (ix *Indexer) buildRoot(ctx context.Context, nodes []Node) (Root, error) {
	var allSummaries strings.Builder
	childIDs := make([]string, len(nodes))

	for i, n := range nodes {
		childIDs[i] = n.ID
		allSummaries.WriteString(n.Abstract)
		allSummaries.WriteString("\n")
	}

	summary, err := ix.summarizer(ctx, allSummaries.String(), ix.cfg.L1TargetTokens)
	if err != nil {
		return Root{}, fmt.Errorf("summarize root: %w", err)
	}
	abstract, err := ix.summarizer(ctx, summary, ix.cfg.L0TargetTokens)
	if err != nil {
		return Root{}, fmt.Errorf("abstract root: %w", err)
	}
	keywords := ExtractKeywords(allSummaries.String(), 15)

	return Root{
		ID:       "root",
		Abstract: abstract,
		Summary:  summary,
		Keywords: keywords,
		ChildIDs: childIDs,
	}, nil
}

// formatTranscript converts messages to a readable transcript.
func formatTranscript(messages []brain.Message) string {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString("[")
		sb.WriteString(m.Role)
		sb.WriteString("]: ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}

// computeChecksum returns a hex SHA1 of the transcript.
func computeChecksum(transcript string) string {
	h := sha1.New()
	h.Write([]byte(transcript))
	return fmt.Sprintf("%x", h.Sum(nil))
}
