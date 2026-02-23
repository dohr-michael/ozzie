package tasks

import (
	"fmt"
	"regexp"
	"strings"
)

// minPlanSteps is the minimum number of steps to consider a parsed plan valid.
const minPlanSteps = 2

// numberedItemRe matches numbered list items like "1. ", "2) ", "1 - " etc.
var numberedItemRe = regexp.MustCompile(`(?m)^(\d+)[.)]\s+(.+)`)

// headerStepRe matches markdown headers like "### Step 1: Title" or "### 1. Title".
var headerStepRe = regexp.MustCompile(`(?m)^###\s+(?:Step\s+)?(\d+)[.:]?\s*(.+)`)

// ParsePlanFromMarkdown extracts a structured TaskPlan from a markdown plan.
// Returns nil if the markdown doesn't contain a recognizable plan (< minPlanSteps steps).
func ParsePlanFromMarkdown(markdown string) *TaskPlan {
	// Try header-based steps first (### Step N: Title)
	if plan := parseHeaderSteps(markdown); plan != nil {
		return plan
	}

	// Fall back to numbered list items (1. Title)
	return parseNumberedSteps(markdown)
}

// parseHeaderSteps extracts steps from "### Step N: Title" patterns.
func parseHeaderSteps(markdown string) *TaskPlan {
	matches := headerStepRe.FindAllStringSubmatchIndex(markdown, -1)
	if len(matches) < minPlanSteps {
		return nil
	}

	var steps []TaskPlanStep
	for i, match := range matches {
		title := strings.TrimSpace(markdown[match[4]:match[5]])

		// Description: text between this header and the next (or end)
		descStart := match[1]
		descEnd := len(markdown)
		if i+1 < len(matches) {
			descEnd = matches[i+1][0]
		}
		desc := strings.TrimSpace(markdown[descStart:descEnd])

		steps = append(steps, TaskPlanStep{
			ID:          fmt.Sprintf("step_%d", i+1),
			Title:       title,
			Description: desc,
			Status:      TaskPending,
		})
	}
	return &TaskPlan{Steps: steps}
}

// parseNumberedSteps extracts steps from "1. Title" patterns.
func parseNumberedSteps(markdown string) *TaskPlan {
	matches := numberedItemRe.FindAllStringSubmatchIndex(markdown, -1)
	if len(matches) < minPlanSteps {
		return nil
	}

	var steps []TaskPlanStep
	for i, match := range matches {
		title := strings.TrimSpace(markdown[match[4]:match[5]])

		// Description: text between this item and the next numbered item (or end)
		descStart := match[1]
		descEnd := len(markdown)
		if i+1 < len(matches) {
			descEnd = matches[i+1][0]
		}
		desc := strings.TrimSpace(markdown[descStart:descEnd])

		steps = append(steps, TaskPlanStep{
			ID:          fmt.Sprintf("step_%d", i+1),
			Title:       title,
			Description: desc,
			Status:      TaskPending,
		})
	}
	return &TaskPlan{Steps: steps}
}
