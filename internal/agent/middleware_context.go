package agent

import (
	"context"
	"fmt"
	"log/slog"
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
	AllToolDescriptions map[string]string // Layer 3: full catalog (name → desc)
	SkillDescriptions   map[string]string // Layer 3b: skill name → description
	Store               sessions.Store    // Session store for metadata
	ToolSet             *ToolSet          // For active/inactive tool lists
	Retriever           MemoryRetriever   // Layer 6: memory retrieval (optional)
}

// NewContextMiddleware builds an AgentMiddleware that injects dynamic context
// (custom instructions, tool descriptions, session context, memories) before
// each chat model call.
func NewContextMiddleware(cfg ContextMiddlewareConfig) adk.AgentMiddleware {
	mw := adk.AgentMiddleware{}

	// AdditionalInstruction: Agent instructions (always) + Layers 2 + 3b (static)
	var instruction strings.Builder

	// Layer 1b: Agent operating instructions (not overridable)
	instruction.WriteString(AgentInstructions)
	instruction.WriteString("\n\n")

	// Layer 2: Custom instructions
	if cfg.CustomInstructions != "" {
		instruction.WriteString("## Additional Instructions\n\n")
		instruction.WriteString(cfg.CustomInstructions)
		instruction.WriteString("\n\n")
	}

	// Layer 3b: Available skills
	if len(cfg.SkillDescriptions) > 0 {
		names := make([]string, 0, len(cfg.SkillDescriptions))
		for name := range cfg.SkillDescriptions {
			names = append(names, name)
		}
		sort.Strings(names)

		instruction.WriteString("## Available Skills\n\n")
		instruction.WriteString("You can delegate complex tasks to these specialized skills:\n")
		for _, name := range names {
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
				sections = append(sections, buildToolSection(activeNames, cfg.AllToolDescriptions))
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

		// Layer 6: Relevant memories
		if cfg.Retriever != nil {
			lastMsg := lastUserMessageContent(state.Messages)
			if lastMsg != "" {
				if memories, err := cfg.Retriever.Retrieve(lastMsg, nil, 5); err == nil && len(memories) > 0 {
					var sb strings.Builder
					sb.WriteString("## Relevant Memories\n\n")
					for _, m := range memories {
						sb.WriteString(fmt.Sprintf("- **[%s] %s**: %s\n", m.Entry.Type, m.Entry.Title, m.Content))
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

func buildToolSection(activeNames []string, allDescs map[string]string) string {
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
