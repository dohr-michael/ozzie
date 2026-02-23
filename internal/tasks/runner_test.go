package tasks

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFormatContextBlock_Empty(t *testing.T) {
	got := formatContextBlock(TaskConfig{})
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestFormatContextBlock_WorkDirOnly(t *testing.T) {
	got := formatContextBlock(TaskConfig{WorkDir: "/home/user/project"})
	if !strings.Contains(got, "Working directory: /home/user/project") {
		t.Errorf("expected working directory, got %q", got)
	}
	if !strings.Contains(got, "do NOT invent paths") {
		t.Errorf("expected warning about paths, got %q", got)
	}
}

func TestFormatContextBlock_WorkDir_NoProtocol(t *testing.T) {
	// Tool Reference and Protocol are now in SubAgentInstructions (middleware),
	// not in formatContextBlock.
	got := formatContextBlock(TaskConfig{WorkDir: "/home/user/project"})
	if strings.Contains(got, "## Protocol") {
		t.Errorf("unexpected Protocol section (moved to SubAgentInstructions), got %q", got)
	}
	if strings.Contains(got, "## Tool Reference") {
		t.Errorf("unexpected Tool Reference section (moved to SubAgentInstructions), got %q", got)
	}
}

func TestFormatContextBlock_NoWorkDir_NoContext(t *testing.T) {
	got := formatContextBlock(TaskConfig{
		Env: map[string]string{"KEY": "val"},
	})
	if strings.Contains(got, "## Execution Context") {
		t.Errorf("expected no Execution Context without WorkDir, got %q", got)
	}
}

func TestFormatContextBlock_EnvOnly(t *testing.T) {
	got := formatContextBlock(TaskConfig{
		Env: map[string]string{"PROJECT_NAME": "chess"},
	})
	if !strings.Contains(got, "- PROJECT_NAME=chess") {
		t.Errorf("expected env var, got %q", got)
	}
}

func TestFormatContextBlock_EnvSorted(t *testing.T) {
	got := formatContextBlock(TaskConfig{
		Env: map[string]string{"ZEBRA": "z", "ALPHA": "a", "MID": "m"},
	})
	alphaIdx := strings.Index(got, "ALPHA=a")
	midIdx := strings.Index(got, "MID=m")
	zebraIdx := strings.Index(got, "ZEBRA=z")
	if alphaIdx < 0 || midIdx < 0 || zebraIdx < 0 {
		t.Fatalf("missing env vars in output: %q", got)
	}
	if !(alphaIdx < midIdx && midIdx < zebraIdx) {
		t.Errorf("env vars not sorted: alpha=%d mid=%d zebra=%d", alphaIdx, midIdx, zebraIdx)
	}
}

func TestFormatContextBlock_Both(t *testing.T) {
	got := formatContextBlock(TaskConfig{
		WorkDir: "/abs/path",
		Env:     map[string]string{"KEY": "val"},
	})
	if !strings.Contains(got, "Working directory: /abs/path") {
		t.Errorf("missing working directory: %q", got)
	}
	if !strings.Contains(got, "- KEY=val") {
		t.Errorf("missing env var: %q", got)
	}
	if strings.Contains(got, "## Protocol") {
		t.Errorf("unexpected Protocol section (moved to SubAgentInstructions): %q", got)
	}
}

// --- buildDependencyContext tests ---

// mockStore implements just enough of Store for testing buildDependencyContext.
type mockStore struct {
	tasks   map[string]*Task
	outputs map[string]string
}

func (m *mockStore) Get(id string) (*Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return t, nil
}

func (m *mockStore) ReadOutput(id string) (string, error) {
	out, ok := m.outputs[id]
	if !ok {
		return "", nil
	}
	return out, nil
}

// Unused Store methods — satisfy interface
func (m *mockStore) Create(*Task) error                              { return nil }
func (m *mockStore) List(ListFilter) ([]*Task, error)                { return nil, nil }
func (m *mockStore) Update(*Task) error                              { return nil }
func (m *mockStore) Delete(string) error                             { return nil }
func (m *mockStore) AppendCheckpoint(string, Checkpoint) error       { return nil }
func (m *mockStore) LoadCheckpoints(string) ([]Checkpoint, error)    { return nil, nil }
func (m *mockStore) WriteOutput(string, string) error                { return nil }
func (m *mockStore) AppendMailbox(string, MailboxMessage) error      { return nil }
func (m *mockStore) LoadMailbox(string) ([]MailboxMessage, error)    { return nil, nil }

func TestBuildDependencyContext_NoDeps(t *testing.T) {
	got := buildDependencyContext(&mockStore{}, nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestBuildDependencyContext_WithOutput(t *testing.T) {
	now := time.Now()
	store := &mockStore{
		tasks: map[string]*Task{
			"task_a": {
				ID:          "task_a",
				Title:       "Setup project",
				Status:      TaskCompleted,
				CompletedAt: &now,
			},
		},
		outputs: map[string]string{
			"task_a": "Created src/main.go and src/board.go",
		},
	}

	got := buildDependencyContext(store, []string{"task_a"})
	if !strings.Contains(got, "## Completed Dependency Tasks") {
		t.Errorf("expected header, got %q", got)
	}
	if !strings.Contains(got, "Setup project") {
		t.Errorf("expected task title, got %q", got)
	}
	if !strings.Contains(got, "Created src/main.go") {
		t.Errorf("expected output content, got %q", got)
	}
	if !strings.Contains(got, "do NOT redo") {
		t.Errorf("expected non-redo directive, got %q", got)
	}
}

func TestBuildDependencyContext_NoOutput(t *testing.T) {
	now := time.Now()
	store := &mockStore{
		tasks: map[string]*Task{
			"task_a": {
				ID:          "task_a",
				Title:       "Silent task",
				Status:      TaskCompleted,
				CompletedAt: &now,
			},
		},
		outputs: map[string]string{},
	}

	got := buildDependencyContext(store, []string{"task_a"})
	if !strings.Contains(got, "Silent task") {
		t.Errorf("expected task title, got %q", got)
	}
	if !strings.Contains(got, "no output captured") {
		t.Errorf("expected no-output marker, got %q", got)
	}
}

func TestBuildDependencyContext_FailedDep(t *testing.T) {
	now := time.Now()
	store := &mockStore{
		tasks: map[string]*Task{
			"task_a": {
				ID:          "task_a",
				Title:       "Broken task",
				Status:      TaskFailed,
				CompletedAt: &now,
				Result:      &TaskResult{Error: "something went wrong"},
			},
		},
		outputs: map[string]string{},
	}

	got := buildDependencyContext(store, []string{"task_a"})
	if !strings.Contains(got, "something went wrong") {
		t.Errorf("expected error message, got %q", got)
	}
}

func TestBuildDependencyContext_TruncatesLongOutput(t *testing.T) {
	now := time.Now()
	longOutput := strings.Repeat("x", maxDependencyOutputLen+500)
	store := &mockStore{
		tasks: map[string]*Task{
			"task_a": {
				ID:          "task_a",
				Title:       "Verbose task",
				Status:      TaskCompleted,
				CompletedAt: &now,
			},
		},
		outputs: map[string]string{
			"task_a": longOutput,
		},
	}

	got := buildDependencyContext(store, []string{"task_a"})
	if !strings.Contains(got, "truncated") {
		t.Errorf("expected truncation marker, got len=%d", len(got))
	}
	if len(got) > maxDependencyOutputLen+500 {
		t.Errorf("output too long: %d", len(got))
	}
}

func TestBuildDependencyContext_MultipleDeps(t *testing.T) {
	now := time.Now()
	store := &mockStore{
		tasks: map[string]*Task{
			"task_a": {ID: "task_a", Title: "First", Status: TaskCompleted, CompletedAt: &now},
			"task_b": {ID: "task_b", Title: "Second", Status: TaskCompleted, CompletedAt: &now},
		},
		outputs: map[string]string{
			"task_a": "output A",
			"task_b": "output B",
		},
	}

	got := buildDependencyContext(store, []string{"task_a", "task_b"})
	if !strings.Contains(got, "First") || !strings.Contains(got, "Second") {
		t.Errorf("expected both task titles, got %q", got)
	}
	if !strings.Contains(got, "output A") || !strings.Contains(got, "output B") {
		t.Errorf("expected both outputs, got %q", got)
	}
}

func TestBuildDependencyContext_MissingDep(t *testing.T) {
	// A dep ID that doesn't exist in the store — should be silently skipped
	store := &mockStore{
		tasks:   map[string]*Task{},
		outputs: map[string]string{},
	}

	got := buildDependencyContext(store, []string{"task_nonexistent"})
	if got != "" {
		t.Errorf("expected empty for missing dep, got %q", got)
	}
}

// --- lastFeedbackStatus tests ---

func TestLastFeedbackStatus_Empty(t *testing.T) {
	got := lastFeedbackStatus(nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestLastFeedbackStatus_Approved(t *testing.T) {
	mailbox := []MailboxMessage{
		{Type: "request", Token: "t1"},
		{Type: "response", Token: "t1", Status: "approved"},
	}
	got := lastFeedbackStatus(mailbox)
	if got != "approved" {
		t.Errorf("expected approved, got %q", got)
	}
}

func TestLastFeedbackStatus_Revise(t *testing.T) {
	mailbox := []MailboxMessage{
		{Type: "request", Token: "t1"},
		{Type: "response", Token: "t1", Status: "approved"},
		{Type: "request", Token: "t2"},
		{Type: "response", Token: "t2", Status: "revise"},
	}
	got := lastFeedbackStatus(mailbox)
	if got != "revise" {
		t.Errorf("expected revise, got %q", got)
	}
}

func TestLastFeedbackStatus_OnlyRequests(t *testing.T) {
	mailbox := []MailboxMessage{
		{Type: "request", Token: "t1"},
		{Type: "exploration"},
	}
	got := lastFeedbackStatus(mailbox)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// --- TaskConfig.IsCoordinator tests ---

func TestTaskConfig_IsCoordinator(t *testing.T) {
	tests := []struct {
		level string
		want  bool
	}{
		{"disabled", false},
		{"supervised", true},
		{"autonomous", true},
		{"", false},
	}
	for _, tt := range tests {
		cfg := TaskConfig{AutonomyLevel: tt.level}
		if got := cfg.IsCoordinator(); got != tt.want {
			t.Errorf("IsCoordinator(%q) = %v, want %v", tt.level, got, tt.want)
		}
	}
}

// --- TaskConfig backward compat ---

func TestTaskConfig_UnmarshalJSON_BackwardCompat(t *testing.T) {
	data := []byte(`{"coordinator": true, "tools": ["cmd"]}`)
	var cfg TaskConfig
	if err := cfg.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if cfg.AutonomyLevel != AutonomySupervised {
		t.Errorf("expected supervised, got %q", cfg.AutonomyLevel)
	}
}

func TestTaskConfig_UnmarshalJSON_NewField(t *testing.T) {
	data := []byte(`{"autonomy_level": "autonomous"}`)
	var cfg TaskConfig
	if err := cfg.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if cfg.AutonomyLevel != AutonomyAutonomous {
		t.Errorf("expected autonomous, got %q", cfg.AutonomyLevel)
	}
}

func TestTaskConfig_UnmarshalJSON_CoordinatorFalse(t *testing.T) {
	data := []byte(`{"coordinator": false}`)
	var cfg TaskConfig
	if err := cfg.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	if cfg.AutonomyLevel != "" {
		t.Errorf("expected empty, got %q", cfg.AutonomyLevel)
	}
}
