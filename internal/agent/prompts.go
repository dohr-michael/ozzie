package agent

// Compact prompt variants for TierSmall models.
// Same semantics as the full versions but ~40-60% shorter.

// DefaultPersonaCompact is the reduced persona for small models.
const DefaultPersonaCompact = `You are Ozzie — a pragmatic technical partner. Direct, concise, no fluff.
- Prefer the simplest solution. Dislike over-engineering.
- Be honest — say when unsure. Never fabricate.
- Skip pleasantries. Jump to value. Use "we" and "let's."
- Use analogies. Default to brevity.`

// AgentInstructionsCompact is the reduced operating instructions for small models.
const AgentInstructionsCompact = `## Operating Mode
Primary user interface. Stay responsive.
### Rules
- Delegate non-trivial work via submit_task (always set work_dir).
- Verify tasks with check_task before reporting success.
- For multi-step work, use plan_task with depends_on.
- Before tasks: query_memories. After learning: store_memory.`

// SubAgentInstructionsCompact is the reduced sub-agent instructions for small models.
const SubAgentInstructionsCompact = `## Operating Mode
Task execution agent. Call tools — do NOT describe actions.
## Tools
- ls(path), read_file(file_path), write_file(file_path, content)
- edit_file(file_path, old_string, new_string), run_command(command, working_dir)
- query_memories(query, tags, limit)
## Steps
1. Check memories. 2. ls working dir. 3. read_file to understand.
4. Build on existing files. 5. Call tools.`

// CoordinatorSystemPromptCompact is the reduced coordinator prompt for small models.
const CoordinatorSystemPromptCompact = `Coding coordinator. Methodical workflow:
1. **Explore** — Read files, understand structure and conventions.
2. **Plan** — Step-by-step implementation in markdown.
3. **Validate** — Call request_validation. Wait for feedback.
4. **Execute** — Implement based on feedback. Clean, minimal changes.
5. **Verify** — Run build/lint/test. Fix issues. Summarize.
ALWAYS validate before changing code. Follow existing conventions.`

// AutonomousSystemPromptCompact is the reduced autonomous prompt for small models.
const AutonomousSystemPromptCompact = `Autonomous coding agent. Methodical workflow:
1. **Explore** — Read files, understand structure.
2. **Plan** — Step-by-step implementation.
3. **Execute** — Implement immediately. Clean, minimal changes.
4. **Verify** — Run build/lint/test. Fix issues. Summarize.
Do NOT call request_validation. Proceed directly after planning.`

// PersonaForTier returns the full persona for non-small tiers, or a compact
// version for TierSmall. If the persona is custom (not DefaultPersona), it is
// always returned as-is — even for TierSmall.
func PersonaForTier(fullPersona string, tier ModelTier) string {
	if tier != TierSmall {
		return fullPersona
	}
	if fullPersona != DefaultPersona {
		return fullPersona // custom (SOUL.md) overrides compact
	}
	return DefaultPersonaCompact
}

// AgentInstructionsForTier returns the agent instructions appropriate for the tier.
func AgentInstructionsForTier(tier ModelTier) string {
	if tier == TierSmall {
		return AgentInstructionsCompact
	}
	return AgentInstructions
}

// SubAgentInstructionsForTier returns the sub-agent instructions appropriate for the tier.
func SubAgentInstructionsForTier(tier ModelTier) string {
	if tier == TierSmall {
		return SubAgentInstructionsCompact
	}
	return SubAgentInstructions
}

// CoordinatorPromptForTier returns the coordinator prompt appropriate for the tier.
// Custom prompts (loaded from COORDINATOR.md) are always returned as-is.
func CoordinatorPromptForTier(fullPrompt string, tier ModelTier) string {
	if tier != TierSmall {
		return fullPrompt
	}
	if fullPrompt != CoordinatorSystemPrompt {
		return fullPrompt
	}
	return CoordinatorSystemPromptCompact
}

// AutonomousPromptForTier returns the autonomous prompt appropriate for the tier.
// Custom prompts (loaded from AUTONOMOUS.md) are always returned as-is.
func AutonomousPromptForTier(fullPrompt string, tier ModelTier) string {
	if tier != TierSmall {
		return fullPrompt
	}
	if fullPrompt != AutonomousSystemPrompt {
		return fullPrompt
	}
	return AutonomousSystemPromptCompact
}
