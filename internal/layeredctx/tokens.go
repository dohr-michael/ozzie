package layeredctx

// EstimateTokens returns a heuristic token count: ~4 chars per token + 4 overhead.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text)/4 + 4
}

// TrimToTokens truncates text to fit within the given token budget.
func TrimToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	maxChars := maxTokens * 4
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars]
}
