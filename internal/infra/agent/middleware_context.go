package agent

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/core/brain"
	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/dohr-michael/ozzie/internal/core/prompt"
	"github.com/dohr-michael/ozzie/internal/infra/sessions"
	"github.com/dohr-michael/ozzie/pkg/memory"
)

// dynamicContextMarker is a sentinel prefix used to make context injection
// idempotent across ReAct iterations and EventRunner retries.
const dynamicContextMarker = "<!-- ozzie:dynamic-context -->\n"

// MemoryRetriever retrieves relevant memories for context injection.
type MemoryRetriever interface {
	Retrieve(ctx context.Context, query string, tags []string, limit int) ([]memory.RetrievedMemory, error)
}

// ContextMiddlewareConfig configures the dynamic context middleware.
type ContextMiddlewareConfig struct {
	CustomInstructions  string              // Layer 2: from config.Agent.SystemPrompt
	PreferredLanguage   string              // Layer 2b: preferred response language (e.g. "en", "fr")
	RuntimeInstruction  string              // Layer 1c: runtime environment (container/local + system tools)
	AllToolDescriptions map[string]string   // Layer 3: full catalog (name → desc)
	SkillDescriptions   map[string]string   // Layer 3b: skill name → description
	Store               sessions.Store      // Session store for metadata
	ToolSet             *brain.ToolSet      // For active/inactive tool lists
	Retriever           MemoryRetriever     // Layer 6: memory retrieval (optional)
	Tier                brain.ModelTier     // Model tier for prompt adaptation
	ActorDescriptions   []prompt.ActorInfo  // Layer 3c: available actors for delegation
}

// NewContextMiddleware builds an AgentMiddleware that injects dynamic context
// (custom instructions, tool descriptions, session context, memories) before
// each chat model call.
func NewContextMiddleware(cfg ContextMiddlewareConfig) adk.AgentMiddleware {
	mw := adk.AgentMiddleware{}

	// AdditionalInstruction: Agent instructions (always) + Layers 2 + 3b (static)
	compact := cfg.Tier == brain.TierSmall
	staticComposer := prompt.NewComposer().
		AddSection("Agent Instructions", AgentInstructionsForTier(cfg.Tier)).
		AddSection("Runtime Environment", cfg.RuntimeInstruction).
		AddSection("Custom Instructions", prompt.CustomInstructionSection(cfg.CustomInstructions)).
		AddSection("Language", prompt.LanguageSection(cfg.PreferredLanguage)).
		AddSection("Skills", prompt.SkillSection(cfg.SkillDescriptions, compact)).
		AddSection("Actors", prompt.ActorSection(cfg.ActorDescriptions))

	if s := staticComposer.String(); s != "" {
		staticComposer.LogManifest("composed static instruction")
		mw.AdditionalInstruction = s
	}

	// BeforeChatModel: Layers 3 (dynamic tools), 4 (session), 6 (memories)
	mw.BeforeChatModel = func(ctx context.Context, state *adk.ChatModelAgentState) error {
		sessionID := events.SessionIDFromContext(ctx)
		dynComposer := prompt.NewComposer()

		// Layer 3: Active/inactive tools (dynamic per session via ToolSet)
		if cfg.ToolSet != nil && sessionID != "" {
			activeNames := cfg.ToolSet.ActiveToolNames(sessionID)
			if len(activeNames) > 0 {
				dynComposer.AddSection("Active Tools", prompt.ToolSection(activeNames, cfg.AllToolDescriptions, compact))
			}
		}

		// Layer 4: Session context
		if cfg.Store != nil && sessionID != "" {
			if sess, err := cfg.Store.Get(sessionID); err == nil {
				section := prompt.SessionSection(sess.RootDir, sess.Language, sess.Title, len(state.Messages))
				dynComposer.AddSection("Session Context", section)
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
				if compact {
					memLimit = 3
					memContentMax = 100
				}

				if memories, err := cfg.Retriever.Retrieve(ctx, query, tags, memLimit); err == nil && len(memories) > 0 {
					// Filter by relevance threshold
					const relevanceThreshold = 0.3
					var infos []prompt.MemoryInfo
					for _, m := range memories {
						if m.Score < relevanceThreshold {
							continue
						}
						infos = append(infos, prompt.MemoryInfo{
							Type:    string(m.Entry.Type),
							Title:   m.Entry.Title,
							Content: m.Content,
						})
					}
					if len(infos) > 0 {
						dynComposer.AddSection("Memories", prompt.MemorySection(infos, memContentMax))
					}
				}
			}
		}

		sections := dynComposer.Sections()
		if len(sections) > 0 {
			content := dynamicContextMarker + dynComposer.String()
			contextMsg := &schema.Message{
				Role:    schema.System,
				Content: content,
			}
			dynComposer.LogManifest("composed dynamic context")
			slog.Debug("dynamic context detail",
				"session_id", sessionID,
				"context_length", len(content),
			)
			// Idempotent: replace existing dynamic context if present
			if len(state.Messages) > 0 &&
				state.Messages[0].Role == schema.System &&
				strings.HasPrefix(state.Messages[0].Content, dynamicContextMarker) {
				state.Messages[0] = contextMsg
			} else {
				state.Messages = append([]*schema.Message{contextMsg}, state.Messages...)
			}
		}

		return nil
	}

	return mw
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

