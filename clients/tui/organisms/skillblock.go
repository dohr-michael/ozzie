package organisms

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/dohr-michael/ozzie/clients/tui/atoms"
)

// SkillStep represents a single step within a skill execution.
type SkillStep struct {
	ID       string
	Title    string
	Complete bool
	Error    string
	Duration time.Duration
}

// SkillBlock displays a skill execution with progress steps.
type SkillBlock struct {
	name     string
	steps    []SkillStep
	complete bool
	errMsg   string
	duration time.Duration
	spinner  atoms.Spinner
	style    lipgloss.Style
	cached   string // rendered view cache
}

// NewSkillBlock creates a skill block.
func NewSkillBlock(name string, style lipgloss.Style) *SkillBlock {
	return &SkillBlock{
		name:    name,
		spinner: atoms.NewSpinner(lipgloss.AdaptiveColor{Light: "#6B21A8", Dark: "#D8A6FF"}),
		style:   style,
	}
}

// AddStep adds or updates a step.
func (sb *SkillBlock) AddStep(id, title string) {
	for i, s := range sb.steps {
		if s.ID == id {
			sb.steps[i].Title = title
			return
		}
	}
	sb.steps = append(sb.steps, SkillStep{ID: id, Title: title})
}

// CompleteStep marks a step as done.
func (sb *SkillBlock) CompleteStep(id string, duration time.Duration, errMsg string) {
	for i, s := range sb.steps {
		if s.ID == id {
			sb.steps[i].Complete = true
			sb.steps[i].Duration = duration
			sb.steps[i].Error = errMsg
			return
		}
	}
}

// SetComplete marks the entire skill as done.
func (sb *SkillBlock) SetComplete(duration time.Duration, errMsg string) {
	sb.complete = true
	sb.duration = duration
	sb.errMsg = errMsg
}

// Name returns the skill name.
func (sb *SkillBlock) Name() string {
	return sb.name
}

// IsComplete returns whether the skill is done.
func (sb *SkillBlock) IsComplete() bool {
	return sb.complete
}

// View renders the skill block.
func (sb *SkillBlock) View() string {
	if sb.complete && sb.cached != "" {
		return sb.cached
	}

	var b strings.Builder

	icon := sb.spinner.View()
	if sb.complete {
		if sb.errMsg != "" {
			icon = "✗"
		} else {
			icon = "✓"
		}
	}

	title := fmt.Sprintf("%s Skill: %s", icon, sb.name)
	if sb.complete && sb.duration > 0 {
		title += fmt.Sprintf(" (%s)", sb.duration.Truncate(time.Millisecond))
	}
	b.WriteString(title)

	for _, step := range sb.steps {
		stepIcon := "  ⠋"
		if step.Complete {
			if step.Error != "" {
				stepIcon = "  ✗"
			} else {
				stepIcon = "  ✓"
			}
		}
		line := fmt.Sprintf("\n%s %s", stepIcon, step.Title)
		if step.Complete && step.Duration > 0 {
			line += fmt.Sprintf(" (%s)", step.Duration.Truncate(time.Millisecond))
		}
		b.WriteString(line)
	}

	if sb.errMsg != "" {
		b.WriteString(fmt.Sprintf("\n  Error: %s", sb.errMsg))
	}

	result := sb.style.Render(b.String())
	if sb.complete {
		sb.cached = result
	}
	return result
}
