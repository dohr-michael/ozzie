package plugins

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/policy"
)

// ApprovePairingTool allows the agent to approve a pairing request.
type ApprovePairingTool struct {
	pairingStore *policy.PairingStore
	resolver     *policy.PolicyResolver
	bus          events.EventBus
}

// NewApprovePairingTool creates a new approve_pairing tool.
func NewApprovePairingTool(store *policy.PairingStore, resolver *policy.PolicyResolver, bus events.EventBus) *ApprovePairingTool {
	return &ApprovePairingTool{
		pairingStore: store,
		resolver:     resolver,
		bus:          bus,
	}
}

// ApprovePairingManifest returns the plugin manifest for the approve_pairing tool.
func ApprovePairingManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "approve_pairing",
		Description: "Approve a user pairing request, granting access with a specific policy",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "approve_pairing",
				Description: "Approve a pairing request from an external platform user. Maps the user to a policy (admin, support, executor, readonly). Use server_id and channel_id to scope the pairing, or omit for platform-wide wildcard.",
				Parameters: map[string]ParamSpec{
					"platform": {
						Type:        "string",
						Description: "Platform name (e.g. \"discord\", \"slack\")",
						Required:    true,
					},
					"user_id": {
						Type:        "string",
						Description: "Platform-specific user ID",
						Required:    true,
					},
					"policy_name": {
						Type:        "string",
						Description: "Policy to apply (admin, support, executor, readonly)",
						Required:    true,
					},
					"server_id": {
						Type:        "string",
						Description: "Server/guild ID (optional, \"*\" for wildcard)",
						Default:     "*",
					},
					"channel_id": {
						Type:        "string",
						Description: "Channel ID (optional, \"*\" for wildcard)",
						Default:     "*",
					},
				},
			},
		},
	}
}

type approvePairingInput struct {
	Platform   string `json:"platform"`
	UserID     string `json:"user_id"`
	PolicyName string `json:"policy_name"`
	ServerID   string `json:"server_id"`
	ChannelID  string `json:"channel_id"`
}

// Info returns the tool info for Eino registration.
func (t *ApprovePairingTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&ApprovePairingManifest().Tools[0]), nil
}

// InvokableRun approves a pairing request.
func (t *ApprovePairingTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input approvePairingInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("approve_pairing: parse input: %w", err)
	}

	if input.Platform == "" {
		return "", fmt.Errorf("approve_pairing: platform is required")
	}
	if input.UserID == "" {
		return "", fmt.Errorf("approve_pairing: user_id is required")
	}
	if input.PolicyName == "" {
		return "", fmt.Errorf("approve_pairing: policy_name is required")
	}

	// Validate policy exists
	if _, ok := t.resolver.Resolve(input.PolicyName); !ok {
		return "", fmt.Errorf("approve_pairing: unknown policy %q (available: %v)", input.PolicyName, t.resolver.Names())
	}

	// Default wildcards
	if input.ServerID == "" {
		input.ServerID = "*"
	}
	if input.ChannelID == "" {
		input.ChannelID = "*"
	}

	// Add pairing
	p := policy.Pairing{
		Key: policy.PairingKey{
			Platform:  input.Platform,
			ServerID:  input.ServerID,
			ChannelID: input.ChannelID,
			UserID:    input.UserID,
		},
		PolicyName: input.PolicyName,
	}
	if err := t.pairingStore.Add(p); err != nil {
		return "", fmt.Errorf("approve_pairing: %w", err)
	}

	// Publish approval event
	t.bus.Publish(events.NewTypedEvent(events.SourceConnector, events.PairingApprovedPayload{
		Platform:   input.Platform,
		ServerID:   input.ServerID,
		ChannelID:  input.ChannelID,
		UserID:     input.UserID,
		PolicyName: input.PolicyName,
	}))

	result := map[string]string{
		"status":      "approved",
		"platform":    input.Platform,
		"user_id":     input.UserID,
		"server_id":   input.ServerID,
		"channel_id":  input.ChannelID,
		"policy_name": input.PolicyName,
	}
	out, _ := json.Marshal(result)
	return string(out), nil
}

var _ tool.InvokableTool = (*ApprovePairingTool)(nil)
