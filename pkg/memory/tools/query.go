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

// QueryMemoriesTool searches memories by query and tags.
type QueryMemoriesTool struct {
	retriever *memory.HybridRetriever
}

// NewQueryMemoriesTool creates a new query_memories tool.
func NewQueryMemoriesTool(retriever *memory.HybridRetriever) *QueryMemoriesTool {
	return &QueryMemoriesTool{retriever: retriever}
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
	return &schema.ToolInfo{
		Name: "query_memories",
		Desc: "Search through stored memories by keyword query and optional tags. Returns the most relevant results.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "Search query keywords",
				Required: true,
			},
			"tags": {
				Type: schema.String,
				Desc: "Comma-separated tags to filter by",
			},
			"limit": {
				Type: schema.Number,
				Desc: "Maximum number of results (default: 5)",
			},
		}),
	}, nil
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
