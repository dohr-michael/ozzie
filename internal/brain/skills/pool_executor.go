package skills

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/dohr-michael/ozzie/internal/brain"
	"github.com/dohr-michael/ozzie/internal/core/events"
)

// PoolSkillExecutor implements tasks.SkillExecutor using the skill registry.
type PoolSkillExecutor struct {
	registry *Registry
	runCfg   RunnerConfig
}

// NewPoolSkillExecutor creates a skill executor backed by the registry.
func NewPoolSkillExecutor(registry *Registry, runCfg RunnerConfig) *PoolSkillExecutor {
	return &PoolSkillExecutor{registry: registry, runCfg: runCfg}
}

// RunSkill executes a skill by name with the given variables.
func (e *PoolSkillExecutor) RunSkill(ctx context.Context, name string, vars map[string]string) (string, error) {
	skill := e.registry.Get(name)
	if skill == nil {
		return "", fmt.Errorf("skill not found: %s", name)
	}

	sessionID := events.SessionIDFromContext(ctx)

	// Emit skill started
	skillType := "instruction"
	if skill.HasWorkflow() {
		skillType = "workflow"
	}
	e.runCfg.EventBus.Publish(events.NewTypedEventWithSession(events.SourceSkill, events.SkillStartedPayload{
		SkillName: skill.Name,
		Type:      skillType,
		Vars:      vars,
	}, sessionID))

	start := time.Now()

	var output string
	var err error
	if skill.HasWorkflow() {
		output, err = e.runWorkflow(ctx, skill, vars)
	} else {
		output, err = e.runSimpleSkill(ctx, skill, vars)
	}

	// Emit skill completed
	payload := events.SkillCompletedPayload{
		SkillName: skill.Name,
		Output:    output,
		Duration:  time.Since(start),
	}
	if err != nil {
		payload.Error = err.Error()
	}
	e.runCfg.EventBus.Publish(events.NewTypedEventWithSession(events.SourceSkill, payload, sessionID))

	return output, err
}

// RunWorkflow executes a named skill's workflow (implements WorkflowExecutor interface for plugins).
func (e *PoolSkillExecutor) RunWorkflow(ctx context.Context, skillName string, vars map[string]string) (string, error) {
	skill := e.registry.Get(skillName)
	if skill == nil {
		return "", fmt.Errorf("skill not found: %s", skillName)
	}
	if !skill.HasWorkflow() {
		return "", fmt.Errorf("skill %q has no workflow", skillName)
	}
	return e.runWorkflow(ctx, skill, vars)
}

func (e *PoolSkillExecutor) runWorkflow(ctx context.Context, skill *SkillMD, vars map[string]string) (string, error) {
	wr, err := NewWorkflowRunnerFromDef(skill, e.runCfg)
	if err != nil {
		return "", err
	}
	return wr.Run(ctx, vars)
}

// runSimpleSkill creates an ephemeral agent with the SKILL.md body as instruction.
func (e *PoolSkillExecutor) runSimpleSkill(ctx context.Context, skill *SkillMD, vars map[string]string) (string, error) {
	// Resolve allowed tools
	tools := e.runCfg.ToolLookup.ToolsByNames(skill.AllowedTools)
	if len(tools) < len(skill.AllowedTools) {
		slog.Warn("some tools not found for skill", "skill", skill.Name, "requested", skill.AllowedTools, "resolved", len(tools))
	}

	// Build instruction from body + vars
	instruction := skill.Body
	if len(vars) > 0 {
		instruction += "\n\n## Variables\n\n"
		for k, v := range vars {
			instruction += fmt.Sprintf("- **%s**: %s\n", k, v)
		}
	}

	// Create ephemeral agent via RunnerFactory (empty model = default)
	runner, err := e.runCfg.RunnerFactory.CreateRunner(ctx, "", instruction, tools)
	if err != nil {
		return "", fmt.Errorf("create agent: %w", err)
	}

	userContent := "Execute this skill."
	if request, ok := vars["request"]; ok && request != "" {
		userContent = request
	}
	messages := []brain.Message{
		{Role: brain.RoleUser, Content: userContent},
	}

	return runner.Run(ctx, messages)
}
