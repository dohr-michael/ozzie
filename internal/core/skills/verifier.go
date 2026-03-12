package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dohr-michael/ozzie/internal/core/brain"
	"github.com/dohr-michael/ozzie/pkg/llmutil"
)

// VerifyResult holds the outcome of a verification check.
type VerifyResult struct {
	Pass     bool     `json:"pass"`
	Issues   []string `json:"issues"`
	Score    int      `json:"score"`
	Feedback string   `json:"feedback"`
}

// Verifier checks step outputs against acceptance criteria using an LLM.
type Verifier struct {
	llmCall brain.SummarizeFunc
}

// NewVerifier creates a new Verifier.
func NewVerifier(llmCall brain.SummarizeFunc) *Verifier {
	return &Verifier{llmCall: llmCall}
}

// Verify checks if the step output meets the acceptance criteria.
func (v *Verifier) Verify(ctx context.Context, criteria *AcceptanceCriteria, stepTitle, output string) (*VerifyResult, error) {
	prompt := buildVerifyPrompt(criteria, stepTitle, output)

	response, err := v.llmCall(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("verifier: generate: %w", err)
	}

	vr := parseVerifyResponse(response)
	return vr, nil
}

func buildVerifyPrompt(criteria *AcceptanceCriteria, stepTitle, output string) string {
	var sb strings.Builder

	sb.WriteString("You are a verification agent. Evaluate whether the following step output meets the acceptance criteria.\n\n")
	sb.WriteString(fmt.Sprintf("## Step: %s\n\n", stepTitle))
	sb.WriteString("## Acceptance Criteria\n\n")
	for i, c := range criteria.Criteria {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, c))
	}
	sb.WriteString("\n## Step Output\n\n")
	// Truncate very long outputs to avoid excessive token usage
	if len(output) > 4000 {
		sb.WriteString(output[:4000])
		sb.WriteString("\n... (truncated)")
	} else {
		sb.WriteString(output)
	}
	sb.WriteString("\n\n## Instructions\n\n")
	sb.WriteString("Respond with a JSON object:\n")
	sb.WriteString("```json\n")
	sb.WriteString(`{"pass": true/false, "score": 0-100, "issues": ["issue1", ...], "feedback": "brief feedback"}`)
	sb.WriteString("\n```\n")
	sb.WriteString("Only output the JSON, no other text.")

	return sb.String()
}

func parseVerifyResponse(content string) *VerifyResult {
	// Try to extract JSON from the response (strip markdown fences if present)
	content = llmutil.StripCodeFences(content)

	var vr VerifyResult
	if err := json.Unmarshal([]byte(content), &vr); err != nil {
		slog.Warn("verifier: failed to parse JSON response, treating as pass", "error", err)
		return &VerifyResult{
			Pass:     true,
			Score:    50,
			Feedback: "Verification response could not be parsed",
		}
	}

	return &vr
}
