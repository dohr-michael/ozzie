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
### Tool Priority
- Prefixed tools (e.g. "system__action") = external APIs via MCP. For read/query: call directly (not submit_task). For write/create with ambiguity: ask user.
- Never answer from memory about external systems. Call the tool.
### Rules
- Delegate multi-step or long work via submit_task (always set work_dir). A single tool call is NOT a task.
- Verify tasks with check_task before reporting success.
- For multi-step work, use plan_task with depends_on.
- Before tasks: query_memories. After learning: store_memory.
- web_search(query): search the web. web_fetch(url): fetch page text.`

// SubAgentInstructionsCompact is the reduced sub-agent instructions for small models.
const SubAgentInstructionsCompact = `## Operating Mode
Task execution agent. Call tools — do NOT describe actions.
## Tools
- ls(path), read_file(file_path), write_file(file_path, content)
- edit_file(file_path, old_string, new_string), run_command(command, working_dir)
- query_memories(query, tags, limit)
## Steps
1. Check memories. 2. ls working dir. 3. read_file to understand.
4. Build on existing files. 5. Call tools.
## File Access
Write ONLY in working dir or shared tmp. No /home, /tmp, /etc.`

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

