package hands

// Tool name constants for unified tools.
const (
	ToolQueryTasks = "query_tasks"
	ToolActivate   = "activate"
	ToolWeb        = "web"
)

// DefaultTaskTools is the default set of tools available to tasks
// when no explicit tools are specified.
var DefaultTaskTools = []string{"run_command", "git"}
