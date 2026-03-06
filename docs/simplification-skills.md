# Simplification: Skills over Coordinator

## Rationale

The coordinator pattern was a Go state machine (~1300 lines across 17+ files) that
orchestrated multi-step tasks through Explore → Plan → Validate → Execute phases.
It has been replaced by a simpler approach: **skills as markdown instructions**.

The main agent can now load a `planner` skill via `activate_skill` to structure its
work when needed. This achieves the same outcome (structured planning) without:

- A rigid state machine in Go code
- Mailbox-based request/response protocol
- Task suspension/resumption mechanics
- Validation channels and token matching
- Custom system prompts per autonomy level

## KISS Principle

The design follows a clear separation:

- **Go code** handles low-level tools: file I/O, command execution, memory, search
- **Skills (SKILL.md)** define high-level behaviors: planning, reviewing, debugging

This makes behaviors user-extensible (edit markdown) rather than requiring Go changes.

## Thinking-Capable Models

This approach relies on the main model being thinking-capable (extended reasoning):
- **Anthropic**: Claude Sonnet 4 / Opus 4
- **Google**: Gemini 2.5 Flash / Pro
- **Mistral**: Mistral Large 3+

A thinking-capable model can self-organize complex tasks without a rigid state machine
enforcing structure. See `docs/llm-evaluation.md` for model requirements.

## DAG Workflows

The DAG workflow engine (`internal/skills/`) is preserved. It serves a different
purpose: ultra-structured execution with sandboxed tools per step, acceptance criteria,
and auto-retry. Useful for operator skills (CI/CD, infrastructure, security) where
deterministic execution order matters more than adaptive reasoning.

## Comparison

| Aspect | Coordinator (removed) | Skills (current) |
|--------|----------------------|-------------------|
| Definition | Go code | SKILL.md + workflow.yaml |
| Extensibility | Requires Go changes | Edit markdown files |
| Validation | Token-based mailbox | Not needed (agent decides) |
| Execution | State machine | Agent follows instructions |
| Maintenance | ~1300 lines | ~30 lines per skill |
