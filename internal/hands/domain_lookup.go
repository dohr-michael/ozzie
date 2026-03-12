package hands

import (
	"github.com/dohr-michael/ozzie/internal/infra/agent"
	"github.com/dohr-michael/ozzie/internal/core/brain"
)

// domainToolLookupAdapter wraps a ToolRegistry as a brain.ToolLookup.
type domainToolLookupAdapter struct {
	registry *ToolRegistry
}

// AsDomainToolLookup returns a brain.ToolLookup view of the registry.
// Each Eino InvokableTool is converted to a brain.Tool on the fly.
func (r *ToolRegistry) AsDomainToolLookup() brain.ToolLookup {
	return &domainToolLookupAdapter{registry: r}
}

// ToolsByNames returns domain tools matching the given names.
func (a *domainToolLookupAdapter) ToolsByNames(names []string) []brain.Tool {
	eino := a.registry.ToolsByNames(names)
	result := make([]brain.Tool, len(eino))
	for i, t := range eino {
		result[i] = agent.WrapEinoTool(t)
	}
	return result
}

// ToolNames returns all registered tool names.
func (a *domainToolLookupAdapter) ToolNames() []string {
	return a.registry.ToolNames()
}

var _ brain.ToolLookup = (*domainToolLookupAdapter)(nil)
