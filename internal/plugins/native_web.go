package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/config"
)

// ---------------------------------------------------------------------------
// Web Search Tool — multi-provider factory
// ---------------------------------------------------------------------------

// WebSearchTool wraps an eino-ext search tool with unified manifest handling.
type WebSearchTool struct {
	inner tool.InvokableTool
}

// NewWebSearchTool creates a web search tool using the configured provider.
// Supported: "duckduckgo" (default, no API key), "google", "bing".
func NewWebSearchTool(ctx context.Context, cfg config.WebSearchConfig) (*WebSearchTool, error) {
	provider := cfg.Provider
	if provider == "" {
		provider = "duckduckgo"
	}

	var inner tool.InvokableTool
	var err error

	switch provider {
	case "duckduckgo":
		inner, err = newDuckDuckGoTool(ctx, cfg)
	case "google":
		inner, err = newGoogleTool(ctx, cfg)
	case "bing":
		inner, err = newBingTool(ctx, cfg)
	default:
		return nil, fmt.Errorf("web_search: unknown provider %q", provider)
	}
	if err != nil {
		return nil, fmt.Errorf("web_search: init %s: %w", provider, err)
	}

	return &WebSearchTool{inner: inner}, nil
}

// Info returns the tool info for Eino registration.
func (t *WebSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.inner.Info(ctx)
}

// InvokableRun delegates to the provider-specific tool.
func (t *WebSearchTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	return t.inner.InvokableRun(ctx, argumentsInJSON, opts...)
}

// WebSearchManifest returns the plugin manifest for web_search.
func WebSearchManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "web_search",
		Description: "Search the web for information using the configured search provider",
		Level:       "tool",
		Provider:    "native",
		Capabilities: CapabilitySet{
			HTTP: &HTTPCapability{AllowedHosts: []string{"*"}},
		},
		Tools: []ToolSpec{
			{
				Name:        "web_search",
				Description: "Search the web for current information. Returns titles, URLs, and snippets.",
				Parameters: map[string]ParamSpec{
					"query": {
						Type:        "string",
						Description: "The search query",
						Required:    true,
					},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Web Fetch Tool — HTTP GET with HTML→text extraction
// ---------------------------------------------------------------------------

// WebFetchTool fetches a URL and returns the text content.
type WebFetchTool struct {
	client    *http.Client
	maxBodyKB int
	userAgent string
}

// NewWebFetchTool creates a web fetch tool with the given config.
func NewWebFetchTool(cfg config.WebFetchConfig) *WebFetchTool {
	timeout := 30 * time.Second
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			timeout = d
		}
	}

	maxBody := cfg.MaxBodyKB
	if maxBody <= 0 {
		maxBody = 512
	}

	ua := cfg.UserAgent
	if ua == "" {
		ua = "Ozzie/1.0 (web_fetch)"
	}

	return &WebFetchTool{
		client:    &http.Client{Timeout: timeout},
		maxBodyKB: maxBody,
		userAgent: ua,
	}
}

type webFetchInput struct {
	URL   string `json:"url"`
	Prompt string `json:"prompt,omitempty"`
}

type webFetchOutput struct {
	URL     string `json:"url"`
	Status  int    `json:"status"`
	Content string `json:"content"`
}

// Info returns the tool info for Eino registration.
func (t *WebFetchTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&WebFetchManifest().Tools[0]), nil
}

// InvokableRun fetches a URL and extracts text content.
func (t *WebFetchTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input webFetchInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("web_fetch: parse input: %w", err)
	}
	if input.URL == "" {
		return "", fmt.Errorf("web_fetch: url is required")
	}

	// Upgrade http to https
	url := input.URL
	if strings.HasPrefix(url, "http://") {
		url = "https://" + strings.TrimPrefix(url, "http://")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("web_fetch: create request: %w", err)
	}
	req.Header.Set("User-Agent", t.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,*/*")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_fetch: %w", err)
	}
	defer resp.Body.Close()

	// Read up to maxBodyKB
	maxBytes := int64(t.maxBodyKB) * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return "", fmt.Errorf("web_fetch: read body: %w", err)
	}

	// Extract text from HTML
	content := extractText(string(body))

	// Truncate to maxBodyKB of text
	if len(content) > int(maxBytes) {
		content = content[:maxBytes]
	}

	result := webFetchOutput{
		URL:     url,
		Status:  resp.StatusCode,
		Content: content,
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("web_fetch: marshal result: %w", err)
	}
	return string(out), nil
}

// WebFetchManifest returns the plugin manifest for web_fetch.
func WebFetchManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "web_fetch",
		Description: "Fetch a web page and extract its text content",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   true,
		Capabilities: CapabilitySet{
			HTTP: &HTTPCapability{AllowedHosts: []string{"*"}},
		},
		Tools: []ToolSpec{
			{
				Name:        "web_fetch",
				Description: "Fetch a URL and return its text content. HTTP URLs are auto-upgraded to HTTPS. Content is truncated to the configured max size.",
				Parameters: map[string]ParamSpec{
					"url": {
						Type:        "string",
						Description: "The URL to fetch",
						Required:    true,
					},
					"prompt": {
						Type:        "string",
						Description: "Optional prompt describing what information to look for on the page",
					},
				},
				Dangerous: true,
			},
		},
	}
}

// extractText strips HTML tags and returns plain text.
// Simple state-machine approach — no external dependency needed.
func extractText(html string) string {
	var sb strings.Builder
	sb.Grow(len(html) / 2)

	inTag := false
	inScript := false
	inStyle := false
	lastSpace := true

	lower := strings.ToLower(html)

	for i := 0; i < len(html); {
		r, size := utf8.DecodeRuneInString(html[i:])

		if inScript {
			if i+9 <= len(lower) && lower[i:i+9] == "</script>" {
				inScript = false
				i += 9
				continue
			}
			i += size
			continue
		}
		if inStyle {
			if i+8 <= len(lower) && lower[i:i+8] == "</style>" {
				inStyle = false
				i += 8
				continue
			}
			i += size
			continue
		}

		if r == '<' {
			// Check for script/style tags
			rest := lower[i:]
			if strings.HasPrefix(rest, "<script") {
				inScript = true
				inTag = true
			} else if strings.HasPrefix(rest, "<style") {
				inStyle = true
				inTag = true
			} else {
				inTag = true
			}

			// Block-level tags → newline
			if len(rest) > 1 {
				tag := rest[1:]
				for _, bt := range []string{"p>", "p ", "div>", "div ", "br>", "br/>", "br />",
					"h1>", "h1 ", "h2>", "h2 ", "h3>", "h3 ", "h4>", "h4 ",
					"li>", "li ", "tr>", "tr ", "td>", "td "} {
					if strings.HasPrefix(tag, bt) || strings.HasPrefix(tag, "/"+bt[:len(bt)-1]) {
						if !lastSpace {
							sb.WriteByte('\n')
							lastSpace = true
						}
						break
					}
				}
			}

			i += size
			continue
		}

		if r == '>' {
			inTag = false
			i += size
			continue
		}

		if inTag {
			i += size
			continue
		}

		// HTML entities
		if r == '&' {
			end := strings.IndexByte(html[i:], ';')
			if end > 0 && end < 10 {
				entity := html[i : i+end+1]
				switch entity {
				case "&nbsp;", "&#160;":
					sb.WriteByte(' ')
				case "&amp;":
					sb.WriteByte('&')
				case "&lt;":
					sb.WriteByte('<')
				case "&gt;":
					sb.WriteByte('>')
				case "&quot;":
					sb.WriteByte('"')
				default:
					sb.WriteString(entity)
				}
				lastSpace = false
				i += end + 1
				continue
			}
		}

		// Collapse whitespace
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !lastSpace {
				sb.WriteByte(' ')
				lastSpace = true
			}
			i += size
			continue
		}

		sb.WriteRune(r)
		lastSpace = false
		i += size
	}

	return strings.TrimSpace(sb.String())
}

// ---------------------------------------------------------------------------
// Provider-specific constructors (lazy imports via eino-ext)
// ---------------------------------------------------------------------------

func newDuckDuckGoTool(ctx context.Context, cfg config.WebSearchConfig) (tool.InvokableTool, error) {
	slog.Info("web_search: using DuckDuckGo provider")

	// Use the eino-ext duckduckgo/v2 package
	duckCfg := &duckduckgoConfig{
		ToolName:   "web_search",
		ToolDesc:   "Search the web using DuckDuckGo. Returns titles, URLs, and summaries.",
		MaxResults: cfg.MaxResults,
	}
	if duckCfg.MaxResults <= 0 {
		duckCfg.MaxResults = 10
	}
	if cfg.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Timeout); err == nil {
			duckCfg.Timeout = d
		}
	}

	return newDuckDuckGoToolFromEino(ctx, duckCfg)
}

func newGoogleTool(ctx context.Context, cfg config.WebSearchConfig) (tool.InvokableTool, error) {
	if cfg.GoogleAPIKey == "" || cfg.GoogleCX == "" {
		return nil, fmt.Errorf("google provider requires google_api_key and google_cx")
	}
	slog.Info("web_search: using Google provider")

	return newGoogleToolFromEino(ctx, cfg)
}

func newBingTool(ctx context.Context, cfg config.WebSearchConfig) (tool.InvokableTool, error) {
	if cfg.BingAPIKey == "" {
		return nil, fmt.Errorf("bing provider requires bing_api_key")
	}
	slog.Info("web_search: using Bing provider")

	return newBingToolFromEino(ctx, cfg)
}

var _ tool.InvokableTool = (*WebSearchTool)(nil)
var _ tool.InvokableTool = (*WebFetchTool)(nil)
