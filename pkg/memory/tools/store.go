package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/pkg/memory"
)

// StoreMemoryTool creates a new memory entry.
type StoreMemoryTool struct {
	store    memory.Store
	pipeline *memory.Pipeline
}

// NewStoreMemoryTool creates a new store_memory tool.
// pipeline may be nil if embeddings are disabled.
func NewStoreMemoryTool(store memory.Store, pipeline *memory.Pipeline) *StoreMemoryTool {
	return &StoreMemoryTool{store: store, pipeline: pipeline}
}

type storeMemoryInput struct {
	Type       string `json:"type"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	Tags       string `json:"tags"`
	Importance string `json:"importance"`
}

func (t *StoreMemoryTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "store_memory",
		Desc: "Store a piece of information in long-term memory for future recall. Use this to remember user preferences, important facts, procedures, or context.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"type": {
				Type:     schema.String,
				Desc:     "Memory type: preference, fact, procedure, or context",
				Required: true,
				Enum:     []string{"preference", "fact", "procedure", "context"},
			},
			"title": {
				Type:     schema.String,
				Desc:     "Short descriptive title for the memory",
				Required: true,
			},
			"content": {
				Type:     schema.String,
				Desc:     "Full content to remember (markdown supported)",
				Required: true,
			},
			"tags": {
				Type: schema.String,
				Desc: "Comma-separated tags for categorization",
			},
			"importance": {
				Type: schema.String,
				Desc: "Importance level: core (never decays), important (slow decay), normal (default), ephemeral (fast decay)",
				Enum: []string{"core", "important", "normal", "ephemeral"},
			},
		}),
	}, nil
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

	importance := memory.ImportanceNormal
	if input.Importance != "" && memory.IsValidImportance(input.Importance) {
		importance = memory.ImportanceLevel(input.Importance)
	}

	entry := &memory.MemoryEntry{
		Title:      input.Title,
		Type:       memory.MemoryType(input.Type),
		Source:     "agent",
		Tags:       tags,
		Importance: importance,
	}

	if err := t.store.Create(entry, input.Content); err != nil {
		return "", fmt.Errorf("store_memory: %w", err)
	}

	// Enqueue embedding job if pipeline is available
	if t.pipeline != nil {
		t.pipeline.Enqueue(memory.EmbedJob{
			ID:      entry.ID,
			Content: memory.BuildEmbedText(entry, input.Content),
			Meta:    memory.BuildEmbedMeta(entry),
		})
	}

	result, _ := json.Marshal(map[string]string{
		"id":     entry.ID,
		"status": "stored",
	})
	return string(result), nil
}

var _ tool.InvokableTool = (*StoreMemoryTool)(nil)
