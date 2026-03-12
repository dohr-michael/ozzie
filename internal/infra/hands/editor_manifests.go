package hands

// StrReplaceEditorManifest returns the plugin manifest for the str_replace_editor tool.
func StrReplaceEditorManifest() *PluginManifest {
	return &PluginManifest{
		Name:        "str_replace_editor",
		Description: "Rich file editor with view, create, str_replace, insert, and undo operations",
		Level:       "tool",
		Provider:    "native",
		Dangerous:   true,
		Capabilities: PluginCapabilities{
			Filesystem: &FSCapabilityIntent{ReadOnly: false},
		},
		Tools: []ToolSpec{
			{
				Name:        "str_replace_editor",
				Description: "A rich file editor. Commands: view (show file with line numbers or list directory), create (create new file), str_replace (replace unique string), insert (insert text after line), undo_edit (undo last edit).",
				Dangerous:   true,
				Parameters: map[string]ParamSpec{
					"command": {
						Type:        "string",
						Description: "The editor command to run",
						Required:    true,
						Enum:        []string{"view", "create", "str_replace", "insert", "undo_edit"},
					},
					"path": {
						Type:        "string",
						Description: "Absolute or relative path to the file or directory",
						Required:    true,
					},
					"file_text": {
						Type:        "string",
						Description: "Content for the 'create' command",
					},
					"old_str": {
						Type:        "string",
						Description: "String to replace (must be unique in the file). Required for 'str_replace'",
					},
					"new_str": {
						Type:        "string",
						Description: "Replacement string. Required for 'str_replace'",
					},
					"insert_line": {
						Type:        "integer",
						Description: "Line number after which to insert (0 = beginning). Required for 'insert'",
					},
					"new_text": {
						Type:        "string",
						Description: "Text to insert. Required for 'insert'",
					},
					"view_range": {
						Type:        "array",
						Description: "Optional [start_line, end_line] for 'view' (1-based inclusive)",
						Items:       &ParamSpec{Type: "integer"},
					},
				},
			},
		},
	}
}
