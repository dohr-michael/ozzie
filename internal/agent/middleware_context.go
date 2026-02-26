package agent

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/memory"
	"github.com/dohr-michael/ozzie/internal/sessions"
)

// MemoryRetriever retrieves relevant memories for context injection.
type MemoryRetriever interface {
	Retrieve(query string, tags []string, limit int) ([]memory.RetrievedMemory, error)
}

// ContextMiddlewareConfig configures the dynamic context middleware.
type ContextMiddlewareConfig struct {
	CustomInstructions  string            // Layer 2: from config.Agent.SystemPrompt
	RuntimeInstruction  string            // Layer 1c: runtime environment (container/local + system tools)
	AllToolDescriptions map[string]string // Layer 3: full catalog (name → desc)
	SkillDescriptions   map[string]string // Layer 3b: skill name → description
	Store               sessions.Store    // Session store for metadata
	ToolSet             *ToolSet          // For active/inactive tool lists
	Retriever           MemoryRetriever   // Layer 6: memory retrieval (optional)
	Tier                ModelTier         // Model tier for prompt adaptation
}

// NewContextMiddleware builds an AgentMiddleware that injects dynamic context
// (custom instructions, tool descriptions, session context, memories) before
// each chat model call.
func NewContextMiddleware(cfg ContextMiddlewareConfig) adk.AgentMiddleware {
	mw := adk.AgentMiddleware{}

	// AdditionalInstruction: Agent instructions (always) + Layers 2 + 3b (static)
	var instruction strings.Builder

	// Layer 1b: Agent operating instructions (not overridable)
	instruction.WriteString(AgentInstructionsForTier(cfg.Tier))
	instruction.WriteString("\n\n")

	// Layer 1c: Runtime environment (container/local + system tools)
	if cfg.RuntimeInstruction != "" {
		instruction.WriteString(cfg.RuntimeInstruction)
		instruction.WriteString("\n\n")
	}

	// Layer 2: Custom instructions
	if cfg.CustomInstructions != "" {
		instruction.WriteString("## Additional Instructions\n\n")
		instruction.WriteString(cfg.CustomInstructions)
		instruction.WriteString("\n\n")
	}

	// Layer 3b: Available skills (limited to 5 for TierSmall)
	if len(cfg.SkillDescriptions) > 0 {
		names := make([]string, 0, len(cfg.SkillDescriptions))
		for name := range cfg.SkillDescriptions {
			names = append(names, name)
		}
		sort.Strings(names)

		maxSkills := len(names)
		if cfg.Tier == TierSmall && maxSkills > 5 {
			maxSkills = 5
		}

		instruction.WriteString("## Available Skills\n\n")
		instruction.WriteString("You can delegate complex tasks to these specialized skills:\n")
		for _, name := range names[:maxSkills] {
			instruction.WriteString(fmt.Sprintf("- **%s**: %s\n", name, cfg.SkillDescriptions[name]))
		}
		instruction.WriteString("\n")
	}

	if s := instruction.String(); s != "" {
		slog.Debug("static additional instruction",
			"length", len(s),
			"instruction", s,
		)
		mw.AdditionalInstruction = s
	}

	// BeforeChatModel: Layers 3 (dynamic tools), 4 (session), 6 (memories)
	mw.BeforeChatModel = func(ctx context.Context, state *adk.ChatModelAgentState) error {
		sessionID := events.SessionIDFromContext(ctx)
		var sections []string

		// Layer 3: Active/inactive tools (dynamic per session via ToolSet)
		if cfg.ToolSet != nil && sessionID != "" {
			activeNames := cfg.ToolSet.ActiveToolNames(sessionID)
			if len(activeNames) > 0 {
				sections = append(sections, buildToolSection(activeNames, cfg.AllToolDescriptions, cfg.Tier))
			}
		}

		// Layer 4: Session context
		if cfg.Store != nil && sessionID != "" {
			if sess, err := cfg.Store.Get(sessionID); err == nil {
				if section := buildSessionSection(sess, len(state.Messages)); section != "" {
					sections = append(sections, section)
				}
			}
		}

		// Layer 6: Relevant memories (enriched with session context)
		if cfg.Retriever != nil {
			lastMsg := lastUserMessageContent(state.Messages)
			if lastMsg != "" {
				query := lastMsg
				var tags []string

				// Enrich query with session context
				if cfg.Store != nil && sessionID != "" {
					if sess, err := cfg.Store.Get(sessionID); err == nil {
						query = enrichQueryWithSession(lastMsg, sess)
						tags = extractSessionTags(sess)
					}
				}

				// Add recent user context for broader semantic match
				if recent := recentUserContext(state.Messages, 2); recent != "" {
					query = query + " " + recent
				}

					memLimit := 5
				memContentMax := 0 // 0 = no truncation
				if cfg.Tier == TierSmall {
					memLimit = 3
					memContentMax = 100
				}

				if memories, err := cfg.Retriever.Retrieve(query, tags, memLimit); err == nil && len(memories) > 0 {
					var sb strings.Builder
					sb.WriteString("## Relevant Memories\n\n")
					for _, m := range memories {
						content := m.Content
						if memContentMax > 0 && len(content) > memContentMax {
							content = content[:memContentMax] + "..."
						}
						sb.WriteString(fmt.Sprintf("- **[%s] %s**: %s\n", m.Entry.Type, m.Entry.Title, content))
					}
					sections = append(sections, sb.String())
				}
			}
		}

		if len(sections) > 0 {
			contextMsg := &schema.Message{
				Role:    schema.System,
				Content: strings.Join(sections, "\n\n"),
			}
			slog.Debug("composed dynamic context",
				"session_id", sessionID,
				"context_length", len(contextMsg.Content),
				"context", contextMsg.Content,
			)
			state.Messages = append([]*schema.Message{contextMsg}, state.Messages...)
		}

		return nil
	}

	return mw
}

func buildToolSection(activeNames []string, allDescs map[string]string, tier ModelTier) string {
	activeSet := make(map[string]bool, len(activeNames))
	sorted := make([]string, len(activeNames))
	copy(sorted, activeNames)
	sort.Strings(sorted)
	for _, n := range sorted {
		activeSet[n] = true
	}

	var sb strings.Builder
	sb.WriteString("## Active Tools (ready to use)\n\n")
	for _, name := range sorted {
		if desc, ok := allDescs[name]; ok && desc != "" {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", name, desc))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s**\n", name))
		}
	}

	// TierSmall: skip inactive tools section to save tokens
	if tier == TierSmall {
		return sb.String()
	}

	var inactive []string
	for name := range allDescs {
		if !activeSet[name] {
			inactive = append(inactive, name)
		}
	}
	sort.Strings(inactive)

	if len(inactive) > 0 {
		sb.WriteString("\n## Available Tools (call activate_tools first)\n\n")
		for _, name := range inactive {
			if desc := allDescs[name]; desc != "" {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", name, desc))
			} else {
				sb.WriteString(fmt.Sprintf("- **%s**\n", name))
			}
		}
	}

	return sb.String()
}

func buildSessionSection(sess *sessions.Session, msgCount int) string {
	var sb strings.Builder
	sb.WriteString("## Session Context\n\n")
	hasContent := false

	if sess.RootDir != "" {
		sb.WriteString(fmt.Sprintf("Working directory: %s\n", sess.RootDir))
		hasContent = true
	}
	if sess.Language != "" {
		sb.WriteString(fmt.Sprintf("Preferred language: %s\n", sess.Language))
		hasContent = true
	}
	if sess.Title != "" && msgCount > 0 {
		sb.WriteString(fmt.Sprintf("Resumed session: %q.\n", sess.Title))
		hasContent = true
	}
	if msgCount > 0 {
		sb.WriteString(fmt.Sprintf("%d previous messages.", msgCount))
		hasContent = true
	}

	if !hasContent {
		return ""
	}
	return sb.String()
}

// lastUserMessageContent returns the content of the last user message.
func lastUserMessageContent(messages []*schema.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == schema.User {
			return messages[i].Content
		}
	}
	return ""
}

// enrichQueryWithSession adds session title and project directory to the query
// for better semantic matching.
func enrichQueryWithSession(query string, sess *sessions.Session) string {
	var parts []string
	parts = append(parts, query)
	if sess.Title != "" {
		parts = append(parts, sess.Title)
	}
	if sess.RootDir != "" {
		parts = append(parts, filepath.Base(sess.RootDir))
	}
	return strings.Join(parts, " ")
}

// extractSessionTags returns tags derived from session metadata.
func extractSessionTags(sess *sessions.Session) []string {
	var tags []string
	if sess.Language != "" {
		tags = append(tags, strings.ToLower(sess.Language))
	}
	if sess.Metadata != nil {
		if project, ok := sess.Metadata["project"]; ok && project != "" {
			tags = append(tags, strings.ToLower(project))
		}
	}
	return tags
}

// recentUserContext returns the concatenated content of the N most recent user
// messages (excluding the last one), each truncated to 200 chars.
func recentUserContext(messages []*schema.Message, maxN int) string {
	var userMsgs []string
	skippedLast := false
	for i := len(messages) - 1; i >= 0 && len(userMsgs) < maxN; i-- {
		if messages[i].Role != schema.User {
			continue
		}
		// Skip the very last user message (already used as primary query)
		if !skippedLast {
			skippedLast = true
			continue
		}
		content := messages[i].Content
		if len(content) > 200 {
			content = content[:200]
		}
		userMsgs = append(userMsgs, content)
	}
	return strings.Join(userMsgs, " ")
}
