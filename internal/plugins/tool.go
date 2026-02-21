package plugins

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	extism "github.com/extism/go-sdk"
)

// WasmTool adapts an Extism WASM plugin to Eino's tool.InvokableTool interface.
// Each WasmTool references a specific ToolSpec; multiple WasmTools may share the
// same extism.Plugin when a plugin exports multiple functions.
type WasmTool struct {
	spec       *ToolSpec      // the specific tool this adapter wraps
	plugin     *extism.Plugin // shared WASM plugin instance
	pluginName string         // plugin name (for error messages)
}

// Info returns the ToolInfo for Eino registration.
func (t *WasmTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(t.spec), nil
}

// InvokableRun calls the WASM export named in spec.Func.
func (t *WasmTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	_, output, err := t.plugin.Call(t.spec.Func, []byte(argumentsInJSON))
	if err != nil {
		return "", fmt.Errorf("plugin %q func %q: %w", t.pluginName, t.spec.Func, err)
	}
	return string(output), nil
}

// toolSpecToToolInfo converts a ToolSpec to an Eino schema.ToolInfo.
func toolSpecToToolInfo(spec *ToolSpec) *schema.ToolInfo {
	info := &schema.ToolInfo{
		Name: spec.Name,
		Desc: spec.Description,
	}

	if len(spec.Parameters) > 0 {
		params := make(map[string]*schema.ParameterInfo, len(spec.Parameters))
		for name, p := range spec.Parameters {
			params[name] = &schema.ParameterInfo{
				Type:     paramTypeToDataType(p.Type),
				Desc:     p.Description,
				Required: p.Required,
				Enum:     p.Enum,
			}
		}
		info.ParamsOneOf = schema.NewParamsOneOfByParams(params)
	}

	return info
}

// paramTypeToDataType maps string type names to Eino DataType constants.
func paramTypeToDataType(t string) schema.DataType {
	switch t {
	case "string":
		return schema.String
	case "number":
		return schema.Number
	case "integer":
		return schema.Integer
	case "boolean":
		return schema.Boolean
	case "array":
		return schema.Array
	case "object":
		return schema.Object
	default:
		return schema.String
	}
}

var _ tool.InvokableTool = (*WasmTool)(nil)
