package hands

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/core/events"
)

// fakeTool is a minimal InvokableTool for testing.
type fakeTool struct {
	called bool
}

func (f *fakeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "fake"}, nil
}

func (f *fakeTool) InvokableRun(_ context.Context, _ string, _ ...tool.Option) (string, error) {
	f.called = true
	return "ok", nil
}

func TestWrapRegistrySandbox_SkipsReadOnly(t *testing.T) {
	bus := events.NewBus(16)
	defer bus.Close()
	registry := NewToolRegistry(bus)

	readTool := &fakeTool{}
	writeTool := &fakeTool{}

	// read_file: Filesystem with ReadOnly=true — should NOT be wrapped
	readManifest := &PluginManifest{
		Name:     "read_file",
		Provider: "native",
		Capabilities: PluginCapabilities{
			Filesystem: &FSCapabilityIntent{ReadOnly: true},
		},
		Tools: []ToolSpec{{Name: "read_file"}},
	}
	readResolved := ResolveCapabilities(readManifest.Capabilities, nil, readManifest.ResourceLimits)
	readManifest.Resolved = &readResolved
	_ = registry.RegisterNative("read_file", readTool, readManifest)

	// write_file: Filesystem without ReadOnly — should be wrapped
	writeManifest := &PluginManifest{
		Name:      "write_file",
		Provider:  "native",
		Dangerous: true,
		Capabilities: PluginCapabilities{
			Filesystem: &FSCapabilityIntent{},
		},
		Tools: []ToolSpec{{Name: "write_file", Dangerous: true}},
	}
	writeResolved := ResolveCapabilities(writeManifest.Capabilities, nil, writeManifest.ResourceLimits)
	writeManifest.Resolved = &writeResolved
	_ = registry.RegisterNative("write_file", writeTool, writeManifest)

	WrapRegistrySandbox(registry, nil)

	// read_file should NOT have been replaced (same pointer)
	if registry.tools["read_file"] != readTool {
		t.Error("read_file (read-only) should not be wrapped by SandboxGuard")
	}

	// write_file should have been replaced (different pointer — wrapped through adapter chain)
	if registry.tools["write_file"] == writeTool {
		t.Error("write_file should be wrapped by SandboxGuard")
	}
}
