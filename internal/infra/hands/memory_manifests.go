package hands

// StoreMemoryManifest returns the plugin manifest for the store_memory tool.
func StoreMemoryManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "store_memory",
		Description: "Store a new long-term memory",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "store_memory",
				Description: "Store a piece of information in long-term memory for future recall. Use this to remember user preferences, important facts, procedures, or context.",
				Parameters: map[string]ParamSpec{
					"type": {
						Type:        "string",
						Description: "Memory type: preference, fact, procedure, or context",
						Required:    true,
						Enum:        []string{"preference", "fact", "procedure", "context"},
					},
					"title": {
						Type:        "string",
						Description: "Short descriptive title for the memory",
						Required:    true,
					},
					"content": {
						Type:        "string",
						Description: "Full content to remember (markdown supported)",
						Required:    true,
					},
					"tags": {
						Type:        "string",
						Description: "Comma-separated tags for categorization",
					},
					"importance": {
						Type:        "string",
						Description: "Importance level: core (never decays), important (slow decay), normal (default), ephemeral (fast decay)",
						Enum:        []string{"core", "important", "normal", "ephemeral"},
					},
				},
			},
		},
	}
}

// QueryMemoriesManifest returns the plugin manifest for the query_memories tool.
func QueryMemoriesManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "query_memories",
		Description: "Search long-term memories",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "query_memories",
				Description: "Search through stored memories by keyword query and optional tags. Returns the most relevant results.",
				Parameters: map[string]ParamSpec{
					"query": {
						Type:        "string",
						Description: "Search query keywords",
						Required:    true,
					},
					"tags": {
						Type:        "string",
						Description: "Comma-separated tags to filter by",
					},
					"limit": {
						Type:        "number",
						Description: "Maximum number of results (default: 5)",
					},
				},
			},
		},
	}
}

// ForgetMemoryManifest returns the plugin manifest for the forget_memory tool.
func ForgetMemoryManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "forget_memory",
		Description: "Delete a memory",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   false,
		Tools: []ToolSpec{
			{
				Name:        "forget_memory",
				Description: "Delete a specific memory entry by its ID. Use this when information is no longer relevant or was stored incorrectly.",
				Parameters: map[string]ParamSpec{
					"id": {
						Type:        "string",
						Description: "The memory ID to delete (e.g., mem_abc12345)",
						Required:    true,
					},
				},
			},
		},
	}
}
