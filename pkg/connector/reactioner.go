package connector

import "context"

// ReactionType represents a semantic reaction kind, independent of any
// platform-specific emoji or icon representation.
type ReactionType string

const (
	ReactionThinking  ReactionType = "thinking"  // LLM reasoning
	ReactionTool      ReactionType = "tool"      // generic tool call
	ReactionWeb       ReactionType = "web"       // web search / fetch
	ReactionCommand   ReactionType = "command"   // shell command execution
	ReactionEdit      ReactionType = "edit"      // file editing
	ReactionTask      ReactionType = "task"      // task management
	ReactionMemory    ReactionType = "memory"    // memory operations
	ReactionSchedule  ReactionType = "schedule"  // scheduling
	ReactionActivate  ReactionType = "activate"  // tool/skill activation
)

// Reactioner is an optional interface that connectors can implement
// to support progress reactions on messages. Each connector maps
// ReactionType to its platform-specific representation (emoji, icon, etc.).
type Reactioner interface {
	AddReaction(ctx context.Context, channelID, messageID string, reaction ReactionType) error
	RemoveReaction(ctx context.Context, channelID, messageID string, reaction ReactionType) error
}
