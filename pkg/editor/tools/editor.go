package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/pkg/editor"
)

// StrReplaceEditorTool wraps pkg/editor as an Eino InvokableTool.
type StrReplaceEditorTool struct {
	editor *editor.Editor
}

// NewStrReplaceEditorTool creates a new str_replace_editor tool.
func NewStrReplaceEditorTool(e *editor.Editor) *StrReplaceEditorTool {
	return &StrReplaceEditorTool{editor: e}
}

type editorInput struct {
	Command    string `json:"command"`
	Path       string `json:"path"`
	FileText   string `json:"file_text"`
	OldStr     string `json:"old_str"`
	NewStr     string `json:"new_str"`
	InsertLine *int   `json:"insert_line"`
	NewText    string `json:"new_text"`
	ViewRange  []int  `json:"view_range"`
}

func (t *StrReplaceEditorTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "str_replace_editor",
		Desc: "A rich file editor. Commands: view (show file with line numbers or list directory), create (create new file), str_replace (replace unique string), insert (insert text after line), undo_edit (undo last edit).",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command": {
				Type:     schema.String,
				Desc:     "The editor command to run",
				Required: true,
				Enum:     []string{"view", "create", "str_replace", "insert", "undo_edit"},
			},
			"path": {
				Type:     schema.String,
				Desc:     "Absolute or relative path to the file or directory",
				Required: true,
			},
			"file_text": {
				Type: schema.String,
				Desc: "Content for the 'create' command",
			},
			"old_str": {
				Type: schema.String,
				Desc: "String to replace (must be unique in the file). Required for 'str_replace'",
			},
			"new_str": {
				Type: schema.String,
				Desc: "Replacement string. Required for 'str_replace'",
			},
			"insert_line": {
				Type: schema.Integer,
				Desc: "Line number after which to insert (0 = beginning). Required for 'insert'",
			},
			"new_text": {
				Type: schema.String,
				Desc: "Text to insert. Required for 'insert'",
			},
			"view_range": {
				Type: schema.Array,
				Desc: "Optional [start_line, end_line] for 'view' (1-based inclusive)",
				ElemInfo: &schema.ParameterInfo{
					Type: schema.Integer,
				},
			},
		}),
	}, nil
}

func (t *StrReplaceEditorTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var input editorInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("str_replace_editor: parse input: %w", err)
	}
	if input.Path == "" {
		return "", fmt.Errorf("str_replace_editor: path is required")
	}

	switch input.Command {
	case "view":
		return t.editor.View(ctx, input.Path, input.ViewRange)
	case "create":
		return t.editor.Create(ctx, input.Path, input.FileText)
	case "str_replace":
		if input.OldStr == "" {
			return "", fmt.Errorf("str_replace_editor: old_str is required for str_replace")
		}
		return t.editor.StrReplace(ctx, input.Path, input.OldStr, input.NewStr)
	case "insert":
		if input.InsertLine == nil {
			return "", fmt.Errorf("str_replace_editor: insert_line is required for insert")
		}
		return t.editor.Insert(ctx, input.Path, *input.InsertLine, input.NewText)
	case "undo_edit":
		return t.editor.UndoEdit(ctx, input.Path)
	default:
		return "", fmt.Errorf("str_replace_editor: unknown command %q", input.Command)
	}
}

var _ tool.InvokableTool = (*StrReplaceEditorTool)(nil)
