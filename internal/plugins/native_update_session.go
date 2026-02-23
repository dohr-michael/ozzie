package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/sessions"
)

// UpdateSessionTool allows the agent to update session metadata at runtime.
type UpdateSessionTool struct {
	store sessions.Store
}

// NewUpdateSessionTool creates a new update_session tool.
func NewUpdateSessionTool(store sessions.Store) *UpdateSessionTool {
	return &UpdateSessionTool{store: store}
}

// UpdateSessionManifest returns the plugin manifest for the update_session tool.
func UpdateSessionManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "update_session",
		Description: "Update the current session metadata (title, language, working directory, etc.)",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "update_session",
				Description: "Update the current session metadata. Only provided fields are updated; omitted fields are left unchanged. Metadata entries are merged (not replaced).",
				Parameters: map[string]ParamSpec{
					"root_dir": {
						Type:        "string",
						Description: "Working directory path for the session",
					},
					"language": {
						Type:        "string",
						Description: "Preferred response language (e.g. \"fr\", \"en\")",
					},
					"title": {
						Type:        "string",
						Description: "Session title",
					},
					"metadata": {
						Type:        "object",
						Description: "Arbitrary key-value metadata to merge into the session",
					},
				},
			},
		},
	}
}

type updateSessionInput struct {
	RootDir  string            `json:"root_dir,omitempty"`
	Language string            `json:"language,omitempty"`
	Title    string            `json:"title,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Info returns the tool info for Eino registration.
func (t *UpdateSessionTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolSpecToToolInfo(&UpdateSessionManifest().Tools[0]), nil
}

// InvokableRun updates the session metadata and returns the updated session.
func (t *UpdateSessionTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	sessionID := events.SessionIDFromContext(ctx)
	if sessionID == "" {
		return "", fmt.Errorf("update_session: no session in context")
	}

	var input updateSessionInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("update_session: parse input: %w", err)
	}

	s, err := t.store.Get(sessionID)
	if err != nil {
		return "", fmt.Errorf("update_session: %w", err)
	}

	// Update only non-empty fields
	if input.RootDir != "" {
		s.RootDir = input.RootDir
	}
	if input.Language != "" {
		s.Language = input.Language
	}
	if input.Title != "" {
		s.Title = input.Title
	}

	// Merge metadata entries
	if len(input.Metadata) > 0 {
		if s.Metadata == nil {
			s.Metadata = make(map[string]string, len(input.Metadata))
		}
		for k, v := range input.Metadata {
			s.Metadata[k] = v
		}
	}

	s.UpdatedAt = time.Now()
	if err := t.store.UpdateMeta(s); err != nil {
		return "", fmt.Errorf("update_session: save: %w", err)
	}

	out, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("update_session: marshal result: %w", err)
	}
	return string(out), nil
}

var _ tool.InvokableTool = (*UpdateSessionTool)(nil)
