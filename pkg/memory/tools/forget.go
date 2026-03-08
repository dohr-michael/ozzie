package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/pkg/memory"
)

// ForgetMemoryTool deletes a memory entry by ID.
type ForgetMemoryTool struct {
	store    memory.Store
	pipeline *memory.Pipeline
}

// NewForgetMemoryTool creates a new forget_memory tool.
// pipeline may be nil if embeddings are disabled.
func NewForgetMemoryTool(store memory.Store, pipeline *memory.Pipeline) *ForgetMemoryTool {
	return &ForgetMemoryTool{store: store, pipeline: pipeline}
}

type forgetMemoryInput struct {
	ID string `json:"id"`
}

func (t *ForgetMemoryTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "forget_memory",
		Desc: "Delete a specific memory entry by its ID. Use this when information is no longer relevant or was stored incorrectly.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {
				Type:     schema.String,
				Desc:     "The memory ID to delete (e.g., mem_abc12345)",
				Required: true,
			},
		}),
	}, nil
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

	// Enqueue vector deletion if pipeline is available
	if t.pipeline != nil {
		t.pipeline.Enqueue(memory.EmbedJob{ID: input.ID, Delete: true})
	}

	result, _ := json.Marshal(map[string]string{
		"id":     input.ID,
		"status": "deleted",
	})
	return string(result), nil
}

var _ tool.InvokableTool = (*ForgetMemoryTool)(nil)
