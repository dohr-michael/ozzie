package tasks

import "context"

// SkillExecutor runs a skill by name. Decouples tasks from the skills package.
type SkillExecutor interface {
	RunSkill(ctx context.Context, skillName string, vars map[string]string) (string, error)
}
