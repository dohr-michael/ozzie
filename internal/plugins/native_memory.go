package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/memory"
)

// =============================================================================
// store_memory
// =============================================================================

// StoreMemoryTool creates a new memory entry.
type StoreMemoryTool struct {
	store memory.Store
}

// NewStoreMemoryTool creates a new store_memory tool.
func NewStoreMemoryTool(store memory.Store) *StoreMemoryTool {
	return &StoreMemoryTool{store: store}
}

// StoreMemoryManifest returns the plugin manifest for the store_memory tool.
func StoreMemoryManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "store_memory",
		Description: "Store a new long-term memory",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "store_memory",
				Description: "Store a piece of information in long-term memory for future recall. Use this to remember user preferences, important facts, procedures, or context.",
				Parameters: map[string]ParamSpec{
					"type": {
						Type:        "string",
						Description: "Memory type: preference, fact, procedure, or context",
						Required:    true,
						Enum:        []string{"preference", "fact", "procedure", "context"},
					},
					"title": {
						Type:        "string",
						Description: "Short descriptive title for the memory",
						Required:    true,
					},
					"content": {
						Type:        "string",
						Description: "Full content to remember (markdown supported)",
						Required:    true,
					},
					"tags": {
						Type:        "string",
						Description: "Comma-separated tags for categorization",
					},
				},
			},
		},
	}
}

type storeMemoryInput struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Tags    string `json:"tags"`
}

func (t *StoreMemoryTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&StoreMemoryManifest().Tools[0]), nil
}

func (t *StoreMemoryTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input storeMemoryInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("store_memory: parse input: %w", err)
	}
	if input.Title == "" {
		return "", fmt.Errorf("store_memory: title is required")
	}
	if input.Content == "" {
		return "", fmt.Errorf("store_memory: content is required")
	}

	var tags []string
	if input.Tags != "" {
		for _, tag := range strings.Split(input.Tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	entry := &memory.MemoryEntry{
		Title:  input.Title,
		Type:   memory.MemoryType(input.Type),
		Source: "agent",
		Tags:   tags,
	}

	if err := t.store.Create(entry, input.Content); err != nil {
		return "", fmt.Errorf("store_memory: %w", err)
	}

	result, _ := json.Marshal(map[string]string{
		"id":     entry.ID,
		"status": "stored",
	})
	return string(result), nil
}

var _ tool.InvokableTool = (*StoreMemoryTool)(nil)

// =============================================================================
// query_memories
// =============================================================================

// QueryMemoriesTool searches memories by query and tags.
type QueryMemoriesTool struct {
	retriever *memory.Retriever
}

// NewQueryMemoriesTool creates a new query_memories tool.
func NewQueryMemoriesTool(retriever *memory.Retriever) *QueryMemoriesTool {
	return &QueryMemoriesTool{retriever: retriever}
}

// QueryMemoriesManifest returns the plugin manifest for the query_memories tool.
func QueryMemoriesManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "query_memories",
		Description: "Search long-term memories",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "query_memories",
				Description: "Search through stored memories by keyword query and optional tags. Returns the most relevant results.",
				Parameters: map[string]ParamSpec{
					"query": {
						Type:        "string",
						Description: "Search query keywords",
						Required:    true,
					},
					"tags": {
						Type:        "string",
						Description: "Comma-separated tags to filter by",
					},
					"limit": {
						Type:        "number",
						Description: "Maximum number of results (default: 5)",
					},
				},
			},
		},
	}
}

type queryMemoriesInput struct {
	Query string  `json:"query"`
	Tags  string  `json:"tags"`
	Limit float64 `json:"limit"`
}

type queryMemoryResult struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Type    string  `json:"type"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func (t *QueryMemoriesTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&QueryMemoriesManifest().Tools[0]), nil
}

func (t *QueryMemoriesTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input queryMemoriesInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("query_memories: parse input: %w", err)
	}

	var tags []string
	if input.Tags != "" {
		for _, tag := range strings.Split(input.Tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	limit := 5
	if input.Limit > 0 {
		limit = int(input.Limit)
	}

	memories, err := t.retriever.Retrieve(input.Query, tags, limit)
	if err != nil {
		return "", fmt.Errorf("query_memories: %w", err)
	}

	var results []queryMemoryResult
	for _, m := range memories {
		results = append(results, queryMemoryResult{
			ID:      m.Entry.ID,
			Title:   m.Entry.Title,
			Type:    string(m.Entry.Type),
			Content: m.Content,
			Score:   m.Score,
		})
	}

	data, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("query_memories: marshal: %w", err)
	}
	return string(data), nil
}

var _ tool.InvokableTool = (*QueryMemoriesTool)(nil)

// =============================================================================
// forget_memory
// =============================================================================

// ForgetMemoryTool deletes a memory entry by ID.
type ForgetMemoryTool struct {
	store memory.Store
}

// NewForgetMemoryTool creates a new forget_memory tool.
func NewForgetMemoryTool(store memory.Store) *ForgetMemoryTool {
	return &ForgetMemoryTool{store: store}
}

// ForgetMemoryManifest returns the plugin manifest for the forget_memory tool.
func ForgetMemoryManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "forget_memory",
		Description: "Delete a memory",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "forget_memory",
				Description: "Delete a specific memory entry by its ID. Use this when information is no longer relevant or was stored incorrectly.",
				Parameters: map[string]ParamSpec{
					"id": {
						Type:        "string",
						Description: "The memory ID to delete (e.g., mem_abc12345)",
						Required:    true,
					},
				},
			},
		},
	}
}

type forgetMemoryInput struct {
	ID string `json:"id"`
}

func (t *ForgetMemoryTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ForgetMemoryManifest().Tools[0]), nil
}

func (t *ForgetMemoryTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input forgetMemoryInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("forget_memory: parse input: %w", err)
	}
	if input.ID == "" {
		return "", fmt.Errorf("forget_memory: id is required")
	}

	if err := t.store.Delete(input.ID); err != nil {
		return "", fmt.Errorf("forget_memory: %w", err)
	}

	result, _ := json.Marshal(map[string]string{
		"id":     input.ID,
		"status": "deleted",
	})
	return string(result), nil
}

var _ tool.InvokableTool = (*ForgetMemoryTool)(nil)
