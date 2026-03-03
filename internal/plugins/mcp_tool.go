package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPTool adapts an external MCP server tool to Eino's tool.InvokableTool interface.
type MCPTool struct {
	serverName string
	toolName   string // original name on the MCP server
	session    *mcp.ClientSession
	mcpTool    *mcp.Tool // full tool definition including InputSchema
	timeout    time.Duration
}

// Info returns the ToolInfo for Eino registration.
func (t *MCPTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	spec := mcpToolToToolSpec(t.serverName, t.mcpTool)
	return toolSpecToToolInfo(&spec), nil
}

// InvokableRun calls the remote MCP tool via the client session.
func (t *MCPTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	var args map[string]any
	if argumentsInJSON != "" && argumentsInJSON != "{}" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", fmt.Errorf("mcp %s__%s: parse args: %w", t.serverName, t.toolName, err)
		}
	}

	result, err := t.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      t.toolName,
		Arguments: args,
	})
	if err != nil {
		return "", fmt.Errorf("mcp %s__%s: call: %w", t.serverName, t.toolName, err)
	}

	if result.IsError {
		text := extractTextContent(result)
		return "", fmt.Errorf("mcp %s__%s: tool error: %s", t.serverName, t.toolName, text)
	}

	return extractTextContent(result), nil
}

// extractTextContent concatenates all TextContent from a CallToolResult.
func extractTextContent(result *mcp.CallToolResult) string {
	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// mcpToolToToolSpec converts an MCP Tool into an Ozzie ToolSpec.
func mcpToolToToolSpec(serverName string, t *mcp.Tool) ToolSpec {
	prefixed := serverName + "__" + t.Name
	spec := ToolSpec{
		Name:        prefixed,
		Description: t.Description,
		Parameters:  make(map[string]ParamSpec),
	}

	// InputSchema is any — typically map[string]any representing JSON Schema
	schemaMap, ok := toMapStringAny(t.InputSchema)
	if !ok {
		return spec
	}

	propsRaw, ok := schemaMap["properties"]
	if !ok {
		return spec
	}
	props, ok := toMapStringAny(propsRaw)
	if !ok {
		return spec
	}

	// Build required set
	requiredSet := make(map[string]bool)
	if reqRaw, ok := schemaMap["required"]; ok {
		if reqSlice, ok := reqRaw.([]any); ok {
			for _, r := range reqSlice {
				if s, ok := r.(string); ok {
					requiredSet[s] = true
				}
			}
		}
	}

	for name, propRaw := range props {
		propMap, ok := toMapStringAny(propRaw)
		if !ok {
			continue
		}
		spec.Parameters[name] = jsonSchemaToParamSpec(propMap, requiredSet[name])
	}

	return spec
}

// jsonSchemaToParamSpec converts a JSON Schema property map to a ParamSpec.
func jsonSchemaToParamSpec(prop map[string]any, required bool) ParamSpec {
	ps := ParamSpec{
		Required: required,
	}

	if t, ok := prop["type"].(string); ok {
		ps.Type = t
	}
	if d, ok := prop["description"].(string); ok {
		ps.Description = d
	}
	if enumRaw, ok := prop["enum"].([]any); ok {
		for _, e := range enumRaw {
			if s, ok := e.(string); ok {
				ps.Enum = append(ps.Enum, s)
			}
		}
	}
	if def, ok := prop["default"]; ok {
		ps.Default = def
	}

	// Array items
	if ps.Type == "array" {
		if itemsRaw, ok := prop["items"]; ok {
			if itemsMap, ok := toMapStringAny(itemsRaw); ok {
				items := jsonSchemaToParamSpec(itemsMap, false)
				ps.Items = &items
			}
		}
	}

	// Object properties (nested)
	if ps.Type == "object" {
		if propsRaw, ok := prop["properties"]; ok {
			if propsMap, ok := toMapStringAny(propsRaw); ok {
				nestedRequired := make(map[string]bool)
				if reqRaw, ok := prop["required"]; ok {
					if reqSlice, ok := reqRaw.([]any); ok {
						for _, r := range reqSlice {
							if s, ok := r.(string); ok {
								nestedRequired[s] = true
							}
						}
					}
				}
				ps.Properties = make(map[string]ParamSpec, len(propsMap))
				for name, subRaw := range propsMap {
					if subMap, ok := toMapStringAny(subRaw); ok {
						ps.Properties[name] = jsonSchemaToParamSpec(subMap, nestedRequired[name])
					}
				}
			}
		}
	}

	return ps
}

// toMapStringAny attempts to convert an any value to map[string]any.
// Handles both map[string]any and JSON-decoded map[string]interface{}.
func toMapStringAny(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	// Try via JSON round-trip for struct types
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	return m, true
}

var _ tool.InvokableTool = (*MCPTool)(nil)
