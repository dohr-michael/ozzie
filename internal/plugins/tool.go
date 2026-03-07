package plugins

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	extism "github.com/extism/go-sdk"

	"github.com/dohr-michael/ozzie/internal/tasks"
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
			params[name] = paramSpecToParameterInfo(p)
		}
		info.ParamsOneOf = schema.NewParamsOneOfByParams(params)
	}

	return info
}

// paramSpecToParameterInfo recursively converts a ParamSpec to an Eino ParameterInfo.
func paramSpecToParameterInfo(p ParamSpec) *schema.ParameterInfo {
	info := &schema.ParameterInfo{
		Type:     paramTypeToDataType(p.Type),
		Desc:     p.Description,
		Required: p.Required,
		Enum:     p.Enum,
	}
	if p.Items != nil {
		info.ElemInfo = paramSpecToParameterInfo(*p.Items)
	}
	if len(p.Properties) > 0 {
		info.SubParams = make(map[string]*schema.ParameterInfo, len(p.Properties))
		for name, sub := range p.Properties {
			info.SubParams[name] = paramSpecToParameterInfo(sub)
		}
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

// enrichActorParamDescriptions rewrites the "actor_tags" and "required_capabilities"
// parameter descriptions in-place, injecting available actors so the LLM knows
// which values are valid. The spec is mutated (callers pass a fresh copy).
func enrichActorParamDescriptions(spec *ToolSpec, actors []tasks.ActorInfo) {
	if len(actors) == 0 {
		return
	}

	// Collect all unique tags and capabilities across actors.
	tagSet := make(map[string]struct{})
	capSet := make(map[string]struct{})
	var actorLines []string
	for _, a := range actors {
		var parts []string
		parts = append(parts, a.ProviderName)
		if len(a.Tags) > 0 {
			parts = append(parts, "tags=["+strings.Join(a.Tags, ",")+"]")
			for _, t := range a.Tags {
				tagSet[t] = struct{}{}
			}
		}
		if len(a.Capabilities) > 0 {
			parts = append(parts, "caps=["+strings.Join(a.Capabilities, ",")+"]")
			for _, c := range a.Capabilities {
				capSet[c] = struct{}{}
			}
		}
		actorLines = append(actorLines, strings.Join(parts, " "))
	}

	if p, ok := spec.Parameters["actor_tags"]; ok {
		var desc strings.Builder
		desc.WriteString("Tags for advanced actor selection. Use tags to target actors based on hosting constraints, data governance, or criticality levels. ")
		desc.WriteString("The task runs on an actor that has ALL specified tags. ")
		if len(tagSet) > 0 {
			tags := sortedKeys(tagSet)
			desc.WriteString("Available tags: [" + strings.Join(tags, ", ") + "]. ")
		} else {
			desc.WriteString("No tags currently configured. ")
		}
		desc.WriteString("Available actors: " + strings.Join(actorLines, "; ") + ".")
		p.Description = desc.String()
		spec.Parameters["actor_tags"] = p
	}

	if p, ok := spec.Parameters["required_capabilities"]; ok {
		var desc strings.Builder
		desc.WriteString("Required model capabilities for task execution. Use capabilities to ensure the actor's model supports specific features (e.g. coding, tool_use, vision). ")
		desc.WriteString("The task runs on an actor whose model has ALL specified capabilities. ")
		if len(capSet) > 0 {
			caps := sortedKeys(capSet)
			desc.WriteString("Available capabilities: [" + strings.Join(caps, ", ") + "]. ")
		} else {
			desc.WriteString("No capabilities currently configured. ")
		}
		p.Description = desc.String()
		spec.Parameters["required_capabilities"] = p
	}
}

// sortedKeys returns the keys of a set sorted alphabetically.
func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple insertion sort — small sets
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
