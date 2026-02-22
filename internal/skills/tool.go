package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/agent"
	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/plugins"
)

// Compile-time check that SkillTool implements tool.InvokableTool.
var _ tool.InvokableTool = (*SkillTool)(nil)

// SkillTool adapts a Skill to Eino's tool.InvokableTool interface.
type SkillTool struct {
	skill  *Skill
	runCfg RunnerConfig
}

// NewSkillTool creates a new skill tool adapter.
func NewSkillTool(skill *Skill, cfg RunnerConfig) *SkillTool {
	return &SkillTool{
		skill:  skill,
		runCfg: cfg,
	}
}

// Info returns the ToolInfo for Eino registration.
func (st *SkillTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	info := &schema.ToolInfo{
		Name: st.skill.Name,
		Desc: st.skill.Description,
	}

	params := make(map[string]*schema.ParameterInfo)

	switch st.skill.Type {
	case SkillTypeSimple:
		// Simple skill: just a "request" parameter
		params["request"] = &schema.ParameterInfo{
			Type:     schema.String,
			Desc:     "The request or input for the skill",
			Required: true,
		}

	case SkillTypeWorkflow:
		// Workflow skill: params from skill.Vars + optional "request"
		for name, v := range st.skill.Vars {
			params[name] = &schema.ParameterInfo{
				Type:     schema.String,
				Desc:     v.Description,
				Required: v.Required,
			}
		}
		params["request"] = &schema.ParameterInfo{
			Type:     schema.String,
			Desc:     "Additional context or request for the workflow",
			Required: false,
		}
	}

	if len(params) > 0 {
		info.ParamsOneOf = schema.NewParamsOneOfByParams(params)
	}

	return info, nil
}

// InvokableRun executes the skill.
func (st *SkillTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	sessionID := events.SessionIDFromContext(ctx)

	// Parse args
	var rawArgs map[string]string
	if err := json.Unmarshal([]byte(argumentsInJSON), &rawArgs); err != nil {
		return "", fmt.Errorf("skill %q: parse args: %w", st.skill.Name, err)
	}

	request := rawArgs["request"]
	delete(rawArgs, "request")

	// Emit skill started
	st.runCfg.EventBus.Publish(events.NewTypedEventWithSession(events.SourceSkill, events.SkillStartedPayload{
		SkillName: st.skill.Name,
		Type:      string(st.skill.Type),
		Vars:      rawArgs,
	}, sessionID))

	start := time.Now()

	var output string
	var err error

	switch st.skill.Type {
	case SkillTypeSimple:
		output, err = st.runSimple(ctx, request)
	case SkillTypeWorkflow:
		// Pass remaining args as vars; add request if non-empty
		vars := rawArgs
		if vars == nil {
			vars = make(map[string]string)
		}
		if request != "" {
			vars["request"] = request
		}
		output, err = st.runWorkflow(ctx, vars)
	default:
		err = fmt.Errorf("skill %q: unknown type %q", st.skill.Name, st.skill.Type)
	}

	// Emit skill completed
	payload := events.SkillCompletedPayload{
		SkillName: st.skill.Name,
		Output:    output,
		Duration:  time.Since(start),
	}
	if err != nil {
		payload.Error = err.Error()
	}
	st.runCfg.EventBus.Publish(events.NewTypedEventWithSession(events.SourceSkill, payload, sessionID))

	if err != nil {
		return "", err
	}
	return output, nil
}

func (st *SkillTool) runSimple(ctx context.Context, request string) (string, error) {
	// Resolve model
	chatModel, err := st.runCfg.ModelRegistry.Get(ctx, st.skill.Model)
	if err != nil {
		chatModel, err = st.runCfg.ModelRegistry.Default(ctx)
		if err != nil {
			return "", fmt.Errorf("resolve model: %w", err)
		}
	}

	// Resolve tools
	var tools []tool.InvokableTool
	for _, name := range st.skill.Tools {
		if t := st.runCfg.ToolRegistry.Tool(name); t != nil {
			tools = append(tools, t)
		}
	}

	// Create ephemeral agent
	runner, err := agent.NewAgent(ctx, chatModel, st.skill.Instruction, tools)
	if err != nil {
		return "", fmt.Errorf("create agent: %w", err)
	}

	// Run with the request as user message
	userContent := request
	if userContent == "" {
		userContent = "Execute this skill."
	}
	messages := []*schema.Message{
		{Role: schema.User, Content: userContent},
	}

	checkpointID := uuid.New().String()
	iter := runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))
	return consumeRunnerOutput(iter)
}

func (st *SkillTool) runWorkflow(ctx context.Context, vars map[string]string) (string, error) {
	wr, err := NewWorkflowRunner(st.skill, st.runCfg)
	if err != nil {
		return "", err
	}
	return wr.Run(ctx, vars)
}

// SkillToManifest converts a Skill to a PluginManifest for ToolRegistry registration.
func SkillToManifest(s *Skill) *plugins.PluginManifest {
	params := make(map[string]plugins.ParamSpec)

	switch s.Type {
	case SkillTypeSimple:
		params["request"] = plugins.ParamSpec{
			Type:        "string",
			Description: "The request or input for the skill",
			Required:    true,
		}

	case SkillTypeWorkflow:
		for name, v := range s.Vars {
			params[name] = plugins.ParamSpec{
				Type:        "string",
				Description: v.Description,
				Required:    v.Required,
			}
		}
		params["request"] = plugins.ParamSpec{
			Type:        "string",
			Description: "Additional context or request for the workflow",
		}
	}

	return &plugins.PluginManifest{
		Name:        s.Name,
		Description: s.Description,
		Level:       "tool",
		Provider:    "skill",
		Tools: []plugins.ToolSpec{
			{
				Name:        s.Name,
				Description: s.Description,
				Parameters:  params,
			},
		},
	}
}
