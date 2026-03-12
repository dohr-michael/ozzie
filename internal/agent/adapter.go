package agent

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/core/brain"
)

// DomainToolAdapter wraps a tool.InvokableTool into a brain.Tool.
type DomainToolAdapter struct {
	inner tool.InvokableTool
}

// WrapEinoTool wraps an Eino InvokableTool as a domain brain.Tool.
func WrapEinoTool(t tool.InvokableTool) brain.Tool {
	return &DomainToolAdapter{inner: t}
}

// Info returns domain-level tool metadata.
func (a *DomainToolAdapter) Info(ctx context.Context) (*brain.ToolInfo, error) {
	info, err := a.inner.Info(ctx)
	if err != nil {
		return nil, err
	}
	return &brain.ToolInfo{
		Name:        info.Name,
		Description: info.Desc,
	}, nil
}

// Run delegates to the Eino tool's InvokableRun.
func (a *DomainToolAdapter) Run(ctx context.Context, argumentsInJSON string) (string, error) {
	return a.inner.InvokableRun(ctx, argumentsInJSON)
}

// EinoToolAdapter wraps a brain.Tool into a tool.InvokableTool.
// It preserves the original Eino ToolInfo for schema generation.
type EinoToolAdapter struct {
	inner    brain.Tool
	einoInfo *schema.ToolInfo // original Eino schema (from the pre-wrapping tool)
}

// UnwrapToEino wraps a domain brain.Tool back into an Eino InvokableTool.
// einoInfo is the original schema to preserve parameter definitions.
func UnwrapToEino(t brain.Tool, einoInfo *schema.ToolInfo) tool.InvokableTool {
	return &EinoToolAdapter{inner: t, einoInfo: einoInfo}
}

// Info returns the preserved Eino ToolInfo.
func (a *EinoToolAdapter) Info(_ context.Context) (*schema.ToolInfo, error) {
	return a.einoInfo, nil
}

// InvokableRun delegates to the domain tool's Run.
func (a *EinoToolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	return a.inner.Run(ctx, argumentsInJSON)
}

// UnwrapEino returns the original InvokableTool if this brain.Tool
// was created by WrapEinoTool. Returns false for non-Eino tools.
func UnwrapEino(t brain.Tool) (tool.InvokableTool, bool) {
	if a, ok := t.(*DomainToolAdapter); ok {
		return a.inner, true
	}
	return nil, false
}

// ConvertToolsToEino converts domain tools to Eino InvokableTools.
// For tools that were originally Eino tools (via WrapEinoTool), the original
// InvokableTool is unwrapped to preserve the full JSON schema.
// For pure domain tools, a minimal Eino wrapper is created.
func ConvertToolsToEino(tools []brain.Tool) []tool.InvokableTool {
	if len(tools) == 0 {
		return nil
	}
	result := make([]tool.InvokableTool, len(tools))
	for i, t := range tools {
		if eino, ok := UnwrapEino(t); ok {
			result[i] = eino
		} else {
			info, _ := t.Info(context.Background())
			name, desc := "", ""
			if info != nil {
				name = info.Name
				desc = info.Description
			}
			result[i] = UnwrapToEino(t, &schema.ToolInfo{Name: name, Desc: desc})
		}
	}
	return result
}

var (
	_ brain.Tool          = (*DomainToolAdapter)(nil)
	_ tool.InvokableTool  = (*EinoToolAdapter)(nil)
)
