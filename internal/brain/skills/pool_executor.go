package skills

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/agent"
	"github.com/dohr-michael/ozzie/internal/events"
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
	// Resolve model
	chatModel, err := e.runCfg.ModelRegistry.Default(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve model: %w", err)
	}

	// Resolve allowed tools
	var tools []tool.InvokableTool
	for _, name := range skill.AllowedTools {
		if t := e.runCfg.ToolRegistry.Tool(name); t != nil {
			tools = append(tools, t)
		} else {
			slog.Warn("tool not found for skill", "skill", skill.Name, "tool", name)
		}
	}

	// Build instruction from body + vars
	instruction := skill.Body
	if len(vars) > 0 {
		instruction += "\n\n## Variables\n\n"
		for k, v := range vars {
			instruction += fmt.Sprintf("- **%s**: %s\n", k, v)
		}
	}

	// Create ephemeral agent
	runner, err := agent.NewAgent(ctx, chatModel, instruction, tools, nil)
	if err != nil {
		return "", fmt.Errorf("create agent: %w", err)
	}

	userContent := "Execute this skill."
	if request, ok := vars["request"]; ok && request != "" {
		userContent = request
	}
	messages := []*schema.Message{
		{Role: schema.User, Content: userContent},
	}

	checkpointID := uuid.New().String()
	iter := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))
	return consumeRunnerOutput(iter)
}
