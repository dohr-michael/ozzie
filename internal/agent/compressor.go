package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/sessions"
)

// SummarizeFunc performs a non-streaming LLM call for summarization.
type SummarizeFunc func(ctx context.Context, prompt string) (string, error)

// CompressorConfig configures context compression behavior.
type CompressorConfig struct {
	ContextWindow int     // total token budget
	Threshold     float64 // trigger ratio (default 0.80)
	PreserveRatio float64 // recent messages budget ratio (default 0.25)
	CharsPerToken int     // heuristic (default 4)
}

// CompressResult holds the output of a compression pass.
type CompressResult struct {
	Messages       []*schema.Message
	NewSummary     string
	NewSummaryUpTo int
	Compressed     bool // true if a new compression occurred
}

// Compressor manages context window compression for long conversations.
type Compressor struct {
	contextWindow int
	threshold     float64
	preserveRatio float64
	charsPerToken int
}

// NewCompressor creates a Compressor with sensible defaults for zero values.
func NewCompressor(cfg CompressorConfig) *Compressor {
	c := &Compressor{
		contextWindow: cfg.ContextWindow,
		threshold:     cfg.Threshold,
		preserveRatio: cfg.PreserveRatio,
		charsPerToken: cfg.CharsPerToken,
	}
	if c.threshold == 0 {
		c.threshold = 0.80
	}
	if c.preserveRatio == 0 {
		c.preserveRatio = 0.25
	}
	if c.charsPerToken == 0 {
		c.charsPerToken = 4
	}
	return c
}

// EstimateTokens returns a heuristic token count for a slice of messages.
func (c *Compressor) EstimateTokens(messages []*schema.Message) int {
	total := 0
	for _, msg := range messages {
		// Content tokens + per-message overhead (~4 tokens for role/formatting)
		total += len(msg.Content)/c.charsPerToken + 4
	}
	return total
}

// NeedsCompression returns true if the total estimated tokens exceed the threshold.
func (c *Compressor) NeedsCompression(systemPromptTokens int, messages []*schema.Message) bool {
	if c.contextWindow <= 0 {
		return false
	}
	msgTokens := c.EstimateTokens(messages)
	used := systemPromptTokens + msgTokens
	limit := int(float64(c.contextWindow) * c.threshold)
	return used > limit
}

// Compress checks whether compression is needed and, if so, summarizes old messages.
// If no compression is needed but the session has an existing summary, it injects
// the summary as the first message.
func (c *Compressor) Compress(
	ctx context.Context,
	session *sessions.Session,
	messages []*schema.Message,
	systemPromptTokens int,
	summarize SummarizeFunc,
) (*CompressResult, error) {
	if !c.NeedsCompression(systemPromptTokens, messages) {
		return c.applyExistingSummary(session, messages), nil
	}

	slog.Info("context compression triggered",
		"messages", len(messages),
		"estimated_tokens", c.EstimateTokens(messages)+systemPromptTokens,
		"context_window", c.contextWindow,
	)

	// Calculate split point: keep recent messages within preserve budget
	preserveBudget := int(float64(c.contextWindow) * c.preserveRatio)
	splitIdx := c.findSplitIndex(messages, preserveBudget)

	oldMessages := messages[:splitIdx]
	recentMessages := messages[splitIdx:]

	// Build summarization prompt
	prompt := c.buildSummarizePrompt(session, oldMessages)

	// Call LLM for summarization
	summary, err := summarize(ctx, prompt)
	if err != nil {
		slog.Error("summarization failed, falling back to truncation", "error", err)
		return c.fallbackTruncate(session, recentMessages), nil
	}

	// Build result with summary message prepended
	summaryMsg := &schema.Message{
		Role:    schema.User,
		Content: fmt.Sprintf("[Previous conversation summary]\n\n%s", summary),
	}

	result := &CompressResult{
		Messages:       append([]*schema.Message{summaryMsg}, recentMessages...),
		NewSummary:     summary,
		NewSummaryUpTo: c.absoluteSummaryIndex(session, splitIdx),
		Compressed:     true,
	}

	slog.Info("context compression complete",
		"old_messages", len(oldMessages),
		"preserved_messages", len(recentMessages),
		"summary_length", len(summary),
	)

	return result, nil
}

// applyExistingSummary injects the session's existing summary (if any) without
// calling the LLM. Returns unmodified messages if no summary exists.
func (c *Compressor) applyExistingSummary(session *sessions.Session, messages []*schema.Message) *CompressResult {
	if session == nil || session.Summary == "" {
		return &CompressResult{Messages: messages}
	}

	summaryMsg := &schema.Message{
		Role:    schema.User,
		Content: fmt.Sprintf("[Previous conversation summary]\n\n%s", session.Summary),
	}

	// Only include messages after the summary point
	remaining := messages
	if session.SummaryUpTo > 0 && session.SummaryUpTo < len(messages) {
		remaining = messages[session.SummaryUpTo:]
	}

	return &CompressResult{
		Messages:       append([]*schema.Message{summaryMsg}, remaining...),
		NewSummary:     session.Summary,
		NewSummaryUpTo: session.SummaryUpTo,
	}
}

// findSplitIndex returns the index that separates old messages from recent ones.
// Recent messages fit within preserveBudget tokens. Always preserves at least 1 message.
func (c *Compressor) findSplitIndex(messages []*schema.Message, preserveBudget int) int {
	if len(messages) <= 1 {
		return 0
	}

	tokens := 0
	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := len(messages[i].Content)/c.charsPerToken + 4
		if tokens+msgTokens > preserveBudget && i < len(messages)-1 {
			return i + 1
		}
		tokens += msgTokens
	}

	// All messages fit in preserve budget but compression was triggered
	// (system prompt is large) — keep at least the last half
	if len(messages) > 1 {
		return len(messages) / 2
	}
	return 0
}

// buildSummarizePrompt constructs the summarization prompt, incorporating
// any previous summary for cumulative compression.
func (c *Compressor) buildSummarizePrompt(session *sessions.Session, oldMessages []*schema.Message) string {
	var sb strings.Builder

	sb.WriteString("You are summarizing a conversation between a user and an AI assistant.\n\n")

	// Cumulative: include previous summary
	if session != nil && session.Summary != "" {
		sb.WriteString("## Previous Summary\n\n")
		sb.WriteString(session.Summary)
		sb.WriteString("\n\n## New Messages to Incorporate\n\n")
	} else {
		sb.WriteString("## Messages\n\n")
	}

	for _, msg := range oldMessages {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n\n", msg.Role, msg.Content))
	}

	sb.WriteString("## Instructions\n\n")
	if session != nil && session.Summary != "" {
		sb.WriteString("Create a new comprehensive summary incorporating both the previous summary and the new messages.\n")
	} else {
		sb.WriteString("Summarize the following conversation.\n")
	}
	sb.WriteString("Produce a structured summary in the conversation's language.\n")
	sb.WriteString("Preserve: key decisions, technical details, file paths, task state, user preferences.\n")
	sb.WriteString("Keep under 2000 words.\n")

	return sb.String()
}

// absoluteSummaryIndex converts a relative split index to an absolute message index,
// accounting for any previous summary offset.
func (c *Compressor) absoluteSummaryIndex(session *sessions.Session, splitIdx int) int {
	if session != nil && session.SummaryUpTo > 0 {
		return session.SummaryUpTo + splitIdx
	}
	return splitIdx
}

// fallbackTruncate drops old messages and injects the existing summary (if any).
// Used when the summarization LLM call fails.
func (c *Compressor) fallbackTruncate(session *sessions.Session, recentMessages []*schema.Message) *CompressResult {
	if session != nil && session.Summary != "" {
		summaryMsg := &schema.Message{
			Role:    schema.User,
			Content: fmt.Sprintf("[Previous conversation summary]\n\n%s", session.Summary),
		}
		return &CompressResult{
			Messages:       append([]*schema.Message{summaryMsg}, recentMessages...),
			NewSummary:     session.Summary,
			NewSummaryUpTo: session.SummaryUpTo,
		}
	}

	return &CompressResult{
		Messages: recentMessages,
	}
}
