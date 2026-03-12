package layered

import (
	"context"
	"fmt"
	"strings"

	"github.com/dohr-michael/ozzie/internal/brain"
)

// Manager orchestrates the full layered context pipeline:
// indexer → retriever → compressed message list.
type Manager struct {
	indexer   *Indexer
	retriever *Retriever
	cfg       Config
}

// NewManager creates a Manager wired to the given store.
// If llm is nil, only the heuristic FallbackSummarizer is used.
func NewManager(store *Store, cfg Config, llm LLMSummarizerFunc) *Manager {
	summarizer := NewLLMSummarizer(llm)
	return &Manager{
		indexer:   NewIndexer(store, summarizer, cfg),
		retriever: NewRetriever(store, cfg),
		cfg:       cfg,
	}
}

// ApplyResult holds stats about a layered context compression pass.
// Nil when no compression was needed.
type ApplyResult struct {
	Escalation   string  // deepest layer reached (L0, L1, L2)
	Nodes        int     // number of archive nodes selected
	Tokens       int     // total tokens used by selected context
	SavingsRatio float64 // 1 - (used / budget)
}

// Apply runs the full layered context pipeline and returns the compressed
// message list ready for the LLM.
//
// Parameters:
//   - sessionID: the session to process
//   - messages: current domain messages (as built from history)
//   - history: the full raw session message history
//
// Returns the modified message list with layered context injected,
// and an ApplyResult with compression stats (nil if no compression).
func (m *Manager) Apply(
	ctx context.Context,
	sessionID string,
	messages []brain.Message,
	history []brain.Message,
) ([]brain.Message, *ApplyResult, error) {
	// Not enough messages to warrant compression
	if len(history) <= m.cfg.MaxRecentMessages {
		return messages, nil, nil
	}

	// Split history: archived + recent
	splitPoint := len(history) - m.cfg.MaxRecentMessages
	archived := history[:splitPoint]
	recent := history[splitPoint:]

	// Build or update the index
	index, err := m.indexer.BuildOrUpdate(ctx, sessionID, archived)
	if err != nil {
		return nil, nil, fmt.Errorf("build index: %w", err)
	}

	// Extract query from the last user message
	query := lastUserMessageContent(messages)

	// Retrieve relevant context
	result, err := m.retriever.Retrieve(sessionID, index, query)
	if err != nil {
		return nil, nil, fmt.Errorf("retrieve: %w", err)
	}

	// Build the layered context message
	contextMsg := buildContextMessage(result)

	// Convert recent history to domain messages
	recentMsgs := make([]brain.Message, 0, len(recent))
	for _, m := range recent {
		if m.Content == "" && m.Role != brain.RoleAssistant {
			continue
		}
		recentMsgs = append(recentMsgs, brain.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Assemble: [context message, ...recent messages]
	out := make([]brain.Message, 0, 1+len(recentMsgs))
	if contextMsg != nil {
		out = append(out, *contextMsg)
	}
	out = append(out, recentMsgs...)

	// Build stats
	ar := &ApplyResult{
		Escalation:   string(result.Decision.ReachedLayer),
		Nodes:        len(result.Selections),
		Tokens:       result.TokenUsage.Used,
		SavingsRatio: result.TokenUsage.SavingsRatio,
	}

	return out, ar, nil
}

// lastUserMessageContent extracts the content of the last user message.
func lastUserMessageContent(messages []brain.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == brain.RoleUser {
			return messages[i].Content
		}
	}
	return ""
}

// buildContextMessage formats retrieved selections into a single context message.
func buildContextMessage(result *RetrievalResult) *brain.Message {
	if result == nil || len(result.Selections) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("[Layered conversation context — retrieved from archived history]\n\n")

	for i, sel := range result.Selections {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		sb.WriteString(fmt.Sprintf("### Archive %s (%s, relevance: %.2f)\n\n", sel.NodeID, sel.Layer, sel.Score))
		sb.WriteString(sel.Content)
		sb.WriteString("\n")
	}

	return &brain.Message{
		Role:    brain.RoleUser,
		Content: sb.String(),
	}
}
