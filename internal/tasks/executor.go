package tasks

import "github.com/dohr-michael/ozzie/internal/brain"

// SkillExecutor runs a skill by name. Decouples tasks from the skills package.
// Canonical definition lives in brain.SkillExecutor.
type SkillExecutor = brain.SkillExecutor

// ToolPermissionsSeeder can seed per-session tool permissions.
// Canonical definition lives in brain.ToolPermissionsSeeder.
type ToolPermissionsSeeder = brain.ToolPermissionsSeeder
