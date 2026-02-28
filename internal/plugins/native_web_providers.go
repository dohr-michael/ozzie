package plugins

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/tool"

	duckduckgo "github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	"github.com/cloudwego/eino-ext/components/tool/bingsearch"
	"github.com/cloudwego/eino-ext/components/tool/googlesearch"

	"github.com/dohr-michael/ozzie/internal/config"
)

// duckduckgoConfig maps to the eino-ext duckduckgo.Config fields we use.
type duckduckgoConfig struct {
	ToolName   string
	ToolDesc   string
	MaxResults int
	Timeout    time.Duration
}

// newDuckDuckGoToolFromEino creates a DuckDuckGo search tool via eino-ext.
func newDuckDuckGoToolFromEino(ctx context.Context, cfg *duckduckgoConfig) (tool.InvokableTool, error) {
	return duckduckgo.NewTextSearchTool(ctx, &duckduckgo.Config{
		ToolName:   cfg.ToolName,
		ToolDesc:   cfg.ToolDesc,
		MaxResults: cfg.MaxResults,
		Timeout:    cfg.Timeout,
	})
}

// newGoogleToolFromEino creates a Google search tool via eino-ext.
func newGoogleToolFromEino(ctx context.Context, cfg config.WebSearchConfig) (tool.InvokableTool, error) {
	num := cfg.MaxResults
	if num <= 0 {
		num = 10
	}
	return googlesearch.NewTool(ctx, &googlesearch.Config{
		APIKey:         cfg.GoogleAPIKey,
		SearchEngineID: cfg.GoogleCX,
		Num:            num,
		ToolName:       "web_search",
		ToolDesc:       "Search the web using Google. Returns titles, URLs, and snippets.",
	})
}

// newBingToolFromEino creates a Bing search tool via eino-ext.
func newBingToolFromEino(ctx context.Context, cfg config.WebSearchConfig) (tool.InvokableTool, error) {
	maxResults := cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}
	bingCfg := &bingsearch.Config{
		APIKey:     cfg.BingAPIKey,
		MaxResults: maxResults,
		ToolName:   "web_search",
		ToolDesc:   "Search the web using Bing. Returns titles, URLs, and descriptions.",
	}
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			bingCfg.Timeout = d
		}
	}
	return bingsearch.NewTool(ctx, bingCfg)
}
