package skills

import (
	"context"
	"encoding/json"
	"fmt"
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
	tool := NewSkillTool(skill, e.runCfg)
	argsJSON, _ := json.Marshal(vars)
	return tool.InvokableRun(ctx, string(argsJSON))
}
