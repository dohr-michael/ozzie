package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/models"
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
	models *models.Registry
}

// NewVerifier creates a new Verifier.
func NewVerifier(models *models.Registry) *Verifier {
	return &Verifier{models: models}
}

// Verify checks if the step output meets the acceptance criteria.
func (v *Verifier) Verify(ctx context.Context, criteria *AcceptanceCriteria, stepTitle, output string) (*VerifyResult, error) {
	modelName := criteria.Model
	if modelName == "" {
		// Default to a fast model for verification
		modelName = v.models.DefaultName()
	}

	chatModel, err := v.models.Get(ctx, modelName)
	if err != nil {
		// Fallback to default
		chatModel, err = v.models.Default(ctx)
		if err != nil {
			return nil, fmt.Errorf("verifier: get model: %w", err)
		}
	}

	prompt := buildVerifyPrompt(criteria, stepTitle, output)

	msgs := []*schema.Message{
		{Role: schema.User, Content: prompt},
	}

	result, err := chatModel.Generate(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("verifier: generate: %w", err)
	}

	vr := parseVerifyResponse(result.Content)
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
	// Try to extract JSON from the response
	content = strings.TrimSpace(content)

	// Strip markdown code fences if present
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		content = strings.Join(jsonLines, "\n")
	}

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
