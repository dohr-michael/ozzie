package layered

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/sessions"
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

// Apply runs the full layered context pipeline and returns the compressed
// message list ready for the LLM.
//
// Parameters:
//   - sessionID: the session to process
//   - messages: current schema messages (as built from history)
//   - history: the full raw session message history
//
// Returns the modified message list with layered context injected.
func (m *Manager) Apply(
	ctx context.Context,
	sessionID string,
	messages []*schema.Message,
	history []sessions.Message,
) ([]*schema.Message, error) {
	// Not enough messages to warrant compression
	if len(history) <= m.cfg.MaxRecentMessages {
		return messages, nil
	}

	// Split history: archived + recent
	splitPoint := len(history) - m.cfg.MaxRecentMessages
	archived := history[:splitPoint]
	recent := history[splitPoint:]

	// Build or update the index
	index, err := m.indexer.BuildOrUpdate(ctx, sessionID, archived)
	if err != nil {
		return nil, fmt.Errorf("build index: %w", err)
	}

	// Extract query from the last user message
	query := lastUserMessageContent(messages)

	// Retrieve relevant context
	result, err := m.retriever.Retrieve(sessionID, index, query)
	if err != nil {
		return nil, fmt.Errorf("retrieve: %w", err)
	}

	// Build the layered context message
	contextMsg := buildContextMessage(result)

	// Convert recent history to schema messages
	recentMsgs := make([]*schema.Message, 0, len(recent))
	for _, m := range recent {
		msg := m.ToSchemaMessage()
		if msg.Content == "" && msg.Role != schema.Assistant {
			continue
		}
		recentMsgs = append(recentMsgs, msg)
	}

	// Assemble: [context message, ...recent messages]
	out := make([]*schema.Message, 0, 1+len(recentMsgs))
	if contextMsg != nil {
		out = append(out, contextMsg)
	}
	out = append(out, recentMsgs...)

	return out, nil
}

// Result returns retrieval metadata from the last Apply call.
// This is useful for emitting events. Returns nil if Apply hasn't been called.
func (m *Manager) LastResult() *RetrievalResult {
	// For now, callers should use the result from Apply directly.
	// This method is a placeholder for future caching.
	return nil
}

// lastUserMessageContent extracts the content of the last user message.
func lastUserMessageContent(messages []*schema.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == schema.User {
			return messages[i].Content
		}
	}
	return ""
}

// buildContextMessage formats retrieved selections into a single context message.
func buildContextMessage(result *RetrievalResult) *schema.Message {
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

	return &schema.Message{
		Role:    schema.User,
		Content: sb.String(),
	}
}
