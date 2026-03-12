package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/sessions"
)

func TestDefaults(t *testing.T) {
	c := NewCompressor(CompressorConfig{})
	if c.threshold != 0.80 {
		t.Errorf("expected threshold 0.80, got %f", c.threshold)
	}
	if c.preserveRatio != 0.25 {
		t.Errorf("expected preserveRatio 0.25, got %f", c.preserveRatio)
	}
	if c.charsPerToken != 4 {
		t.Errorf("expected charsPerToken 4, got %d", c.charsPerToken)
	}
}

func TestEstimateTokens(t *testing.T) {
	c := NewCompressor(CompressorConfig{CharsPerToken: 4})

	messages := []*schema.Message{
		{Role: schema.User, Content: "Hello world"},       // 11 chars → 2 + 4 overhead = 6
		{Role: schema.Assistant, Content: "Hi there mate"}, // 13 chars → 3 + 4 overhead = 7
	}

	tokens := c.EstimateTokens(messages)
	// (11/4 + 4) + (13/4 + 4) = 6 + 7 = 13
	if tokens != 13 {
		t.Errorf("expected 13 tokens, got %d", tokens)
	}
}

func TestEstimateTokens_Empty(t *testing.T) {
	c := NewCompressor(CompressorConfig{})
	tokens := c.EstimateTokens(nil)
	if tokens != 0 {
		t.Errorf("expected 0 tokens for nil messages, got %d", tokens)
	}
}

func TestNeedsCompression(t *testing.T) {
	c := NewCompressor(CompressorConfig{
		ContextWindow: 100,
		Threshold:     0.80,
		CharsPerToken: 4,
	})

	// Under threshold
	smallMsg := []*schema.Message{{Role: schema.User, Content: "hi"}}
	if c.NeedsCompression(0, smallMsg) {
		t.Error("should not need compression for small messages")
	}

	// Over threshold: 80 tokens of system + messages that push past 80% of 100
	bigContent := strings.Repeat("x", 400) // 400/4 + 4 = 104 tokens
	bigMsg := []*schema.Message{{Role: schema.User, Content: bigContent}}
	if !c.NeedsCompression(0, bigMsg) {
		t.Error("should need compression for large messages")
	}
}

func TestNeedsCompression_ZeroWindow(t *testing.T) {
	c := NewCompressor(CompressorConfig{ContextWindow: 0})
	bigMsg := []*schema.Message{{Role: schema.User, Content: strings.Repeat("x", 10000)}}
	if c.NeedsCompression(0, bigMsg) {
		t.Error("should never need compression when context window is 0")
	}
}

func TestCompress_NoCompressionNeeded(t *testing.T) {
	c := NewCompressor(CompressorConfig{
		ContextWindow: 10000,
		CharsPerToken: 4,
	})

	messages := []*schema.Message{
		{Role: schema.User, Content: "Hello"},
		{Role: schema.Assistant, Content: "Hi there"},
	}

	result, err := c.Compress(context.Background(), nil, messages, 10, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Compressed {
		t.Error("should not compress small conversation")
	}
	if len(result.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result.Messages))
	}
}

func TestCompress_ApplyExistingSummary(t *testing.T) {
	c := NewCompressor(CompressorConfig{
		ContextWindow: 10000,
		CharsPerToken: 4,
	})

	session := &sessions.Session{
		Summary:     "Previous conversation about Go testing",
		SummaryUpTo: 5,
	}

	// 10 messages, but summary covers first 5
	messages := make([]*schema.Message, 10)
	for i := range messages {
		messages[i] = &schema.Message{Role: schema.User, Content: fmt.Sprintf("msg %d", i)}
	}

	called := false
	mockSummarize := func(_ context.Context, _ string) (string, error) {
		called = true
		return "", nil
	}

	result, err := c.Compress(context.Background(), session, messages, 10, mockSummarize)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("should not call summarize when no compression needed")
	}
	if result.Compressed {
		t.Error("should not flag as newly compressed")
	}

	// Should have summary message + messages[5:]
	expectedLen := 1 + 5 // summary + remaining
	if len(result.Messages) != expectedLen {
		t.Errorf("expected %d messages, got %d", expectedLen, len(result.Messages))
	}

	if !strings.Contains(result.Messages[0].Content, "Previous conversation about Go testing") {
		t.Error("summary message should contain existing summary")
	}
}

func TestCompress_TriggersCompression(t *testing.T) {
	c := NewCompressor(CompressorConfig{
		ContextWindow: 100,
		Threshold:     0.50, // low threshold to trigger easily
		PreserveRatio: 0.25,
		CharsPerToken: 4,
	})

	// Create enough messages to exceed threshold
	messages := make([]*schema.Message, 20)
	for i := range messages {
		messages[i] = &schema.Message{
			Role:    schema.User,
			Content: fmt.Sprintf("This is message number %d with some content", i),
		}
	}

	var promptReceived string
	mockSummarize := func(_ context.Context, prompt string) (string, error) {
		promptReceived = prompt
		return "Summary of the conversation about numbered messages.", nil
	}

	result, err := c.Compress(context.Background(), nil, messages, 10, mockSummarize)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Compressed {
		t.Error("should flag as compressed")
	}
	if result.NewSummary == "" {
		t.Error("should have a new summary")
	}
	if promptReceived == "" {
		t.Error("summarize should have been called")
	}

	// First message should be the summary
	if !strings.Contains(result.Messages[0].Content, "[Previous conversation summary]") {
		t.Error("first message should be the summary injection")
	}
	if result.Messages[0].Role != schema.User {
		t.Errorf("summary message role should be User, got %s", result.Messages[0].Role)
	}

	// Should preserve some recent messages
	if len(result.Messages) < 2 {
		t.Error("should preserve at least 1 recent message plus summary")
	}
}

func TestCompress_CumulativeSummary(t *testing.T) {
	c := NewCompressor(CompressorConfig{
		ContextWindow: 100,
		Threshold:     0.50,
		PreserveRatio: 0.25,
		CharsPerToken: 4,
	})

	session := &sessions.Session{
		Summary:     "Earlier: user discussed Go project setup.",
		SummaryUpTo: 10,
	}

	messages := make([]*schema.Message, 20)
	for i := range messages {
		messages[i] = &schema.Message{
			Role:    schema.User,
			Content: fmt.Sprintf("Message %d about context compression", i),
		}
	}

	var promptReceived string
	mockSummarize := func(_ context.Context, prompt string) (string, error) {
		promptReceived = prompt
		return "Updated comprehensive summary.", nil
	}

	result, err := c.Compress(context.Background(), session, messages, 10, mockSummarize)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(promptReceived, "Previous Summary") {
		t.Error("cumulative prompt should contain 'Previous Summary' section")
	}
	if !strings.Contains(promptReceived, "Earlier: user discussed Go project setup.") {
		t.Error("cumulative prompt should include previous summary text")
	}
	if !result.Compressed {
		t.Error("should flag as compressed")
	}

	// SummaryUpTo should be offset by previous summary
	if result.NewSummaryUpTo <= session.SummaryUpTo {
		t.Errorf("new SummaryUpTo (%d) should be greater than previous (%d)",
			result.NewSummaryUpTo, session.SummaryUpTo)
	}
}

func TestCompress_SummarizationFailure(t *testing.T) {
	c := NewCompressor(CompressorConfig{
		ContextWindow: 100,
		Threshold:     0.50,
		PreserveRatio: 0.25,
		CharsPerToken: 4,
	})

	messages := make([]*schema.Message, 20)
	for i := range messages {
		messages[i] = &schema.Message{
			Role:    schema.User,
			Content: fmt.Sprintf("Message %d with enough content to trigger", i),
		}
	}

	mockSummarize := func(_ context.Context, _ string) (string, error) {
		return "", fmt.Errorf("LLM unavailable")
	}

	result, err := c.Compress(context.Background(), nil, messages, 10, mockSummarize)
	if err != nil {
		t.Fatalf("should not return error on summarization failure (fallback): %v", err)
	}
	if result.Compressed {
		t.Error("should not flag as compressed on fallback")
	}

	// Should still have some messages (truncated)
	if len(result.Messages) == 0 {
		t.Error("fallback should preserve recent messages")
	}
	if len(result.Messages) >= 20 {
		t.Error("fallback should have fewer messages than original")
	}
}

func TestCompress_PreserveMinOne(t *testing.T) {
	c := NewCompressor(CompressorConfig{
		ContextWindow: 10,  // very small
		Threshold:     0.1, // very aggressive
		PreserveRatio: 0.01,
		CharsPerToken: 4,
	})

	messages := []*schema.Message{
		{Role: schema.User, Content: strings.Repeat("x", 200)},
	}

	mockSummarize := func(_ context.Context, _ string) (string, error) {
		return "Summary", nil
	}

	result, err := c.Compress(context.Background(), nil, messages, 0, mockSummarize)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With only 1 message, findSplitIndex returns 0 → nothing to summarize
	// The single message should be preserved
	if len(result.Messages) == 0 {
		t.Error("should always preserve at least 1 message")
	}
}
