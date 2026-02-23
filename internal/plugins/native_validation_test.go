package plugins

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/tasks"
)

func TestRequestValidationTool_NoChannel(t *testing.T) {
	tool := NewRequestValidationTool()
	input, _ := json.Marshal(map[string]string{"plan": "my plan"})

	_, err := tool.InvokableRun(context.Background(), string(input))
	if err == nil {
		t.Fatal("expected error when no validation channel in context")
	}
}

func TestRequestValidationTool_Success(t *testing.T) {
	tool := NewRequestValidationTool()
	ch := make(chan events.ValidationRequest, 1)
	ctx := events.ContextWithValidationCh(context.Background(), ch)

	input, _ := json.Marshal(map[string]string{"plan": "## Plan\n1. Do something"})
	result, err := tool.InvokableRun(ctx, string(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]string
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out["status"] != "suspended" {
		t.Errorf("expected status=suspended, got %s", out["status"])
	}
	if out["token"] == "" {
		t.Error("expected non-empty token")
	}

	// Verify signal was sent
	select {
	case req := <-ch:
		if req.Token != out["token"] {
			t.Errorf("token mismatch: got %s, want %s", req.Token, out["token"])
		}
		if req.Content != "## Plan\n1. Do something" {
			t.Errorf("content mismatch: got %q", req.Content)
		}
	default:
		t.Error("expected validation request on channel")
	}
}

func TestRequestValidationTool_EmptyPlan(t *testing.T) {
	tool := NewRequestValidationTool()
	ch := make(chan events.ValidationRequest, 1)
	ctx := events.ContextWithValidationCh(context.Background(), ch)

	input, _ := json.Marshal(map[string]string{"plan": ""})
	_, err := tool.InvokableRun(ctx, string(input))
	if err == nil {
		t.Fatal("expected error for empty plan")
	}
}

// mockSubmitter implements tasks.TaskSubmitter for testing.
type mockSubmitter struct {
	store      tasks.Store
	resumed    []string
	cancelledID string
}

func (m *mockSubmitter) Submit(_ *tasks.Task) error           { return nil }
func (m *mockSubmitter) Cancel(id string, _ string) error     { m.cancelledID = id; return nil }
func (m *mockSubmitter) ResumeTask(id string) error           { m.resumed = append(m.resumed, id); return nil }
func (m *mockSubmitter) Store() tasks.Store                   { return m.store }

func TestReplyTaskTool_Success(t *testing.T) {
	dir := t.TempDir()
	store := tasks.NewFileStore(dir)

	// Create a suspended task waiting for reply
	tk := &tasks.Task{
		ID:              "task_abc",
		SessionID:       "sess_1",
		Title:           "Test task",
		Description:     "test",
		Status:          tasks.TaskSuspended,
		Priority:        tasks.PriorityNormal,
		WaitingForReply: true,
	}
	if err := store.Create(tk); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Add a mailbox request
	msg := tasks.MailboxMessage{
		ID:    "msg_1",
		Ts:    time.Now(),
		Type:  "request",
		Token: "tok_123",
		Content: "Here is my plan",
	}
	if err := store.AppendMailbox("task_abc", msg); err != nil {
		t.Fatalf("append mailbox: %v", err)
	}

	sub := &mockSubmitter{store: store}
	tool := NewReplyTaskTool(sub)

	input, _ := json.Marshal(map[string]string{
		"task_id":  "task_abc",
		"feedback": "Looks good, proceed!",
		"status":   "approved",
	})

	result, err := tool.InvokableRun(context.Background(), string(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]string
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["status"] != "resumed" {
		t.Errorf("expected status=resumed, got %s", out["status"])
	}

	// Verify mailbox has the response
	mailbox, err := store.LoadMailbox("task_abc")
	if err != nil {
		t.Fatalf("load mailbox: %v", err)
	}
	if len(mailbox) != 2 {
		t.Fatalf("expected 2 mailbox messages, got %d", len(mailbox))
	}
	resp := mailbox[1]
	if resp.Type != "response" {
		t.Errorf("expected type=response, got %s", resp.Type)
	}
	if resp.Token != "tok_123" {
		t.Errorf("expected token=tok_123, got %s", resp.Token)
	}
	if resp.Content != "Looks good, proceed!" {
		t.Errorf("expected feedback content, got %q", resp.Content)
	}
	if resp.Status != "approved" {
		t.Errorf("expected status=approved, got %q", resp.Status)
	}

	// Verify ResumeTask was called
	if len(sub.resumed) != 1 || sub.resumed[0] != "task_abc" {
		t.Errorf("expected ResumeTask called with task_abc, got %v", sub.resumed)
	}
}

func TestReplyTaskTool_NotSuspended(t *testing.T) {
	dir := t.TempDir()
	store := tasks.NewFileStore(dir)

	tk := &tasks.Task{
		ID:       "task_xyz",
		Title:    "Running task",
		Status:   tasks.TaskRunning,
		Priority: tasks.PriorityNormal,
	}
	if err := store.Create(tk); err != nil {
		t.Fatalf("create task: %v", err)
	}

	sub := &mockSubmitter{store: store}
	tool := NewReplyTaskTool(sub)

	input, _ := json.Marshal(map[string]string{
		"task_id":  "task_xyz",
		"feedback": "go",
	})

	_, err := tool.InvokableRun(context.Background(), string(input))
	if err == nil {
		t.Fatal("expected error for non-suspended task")
	}
}

func TestReplyTaskTool_NotWaitingForReply(t *testing.T) {
	dir := t.TempDir()
	store := tasks.NewFileStore(dir)

	tk := &tasks.Task{
		ID:              "task_xyz",
		Title:           "Suspended task",
		Status:          tasks.TaskSuspended,
		Priority:        tasks.PriorityNormal,
		WaitingForReply: false,
	}
	if err := store.Create(tk); err != nil {
		t.Fatalf("create task: %v", err)
	}

	sub := &mockSubmitter{store: store}
	tool := NewReplyTaskTool(sub)

	input, _ := json.Marshal(map[string]string{
		"task_id":  "task_xyz",
		"feedback": "go",
	})

	_, err := tool.InvokableRun(context.Background(), string(input))
	if err == nil {
		t.Fatal("expected error for task not waiting for reply")
	}
}

func TestFindPendingRequestToken(t *testing.T) {
	tests := []struct {
		name    string
		mailbox []tasks.MailboxMessage
		want    string
	}{
		{
			name:    "empty mailbox",
			mailbox: nil,
			want:    "",
		},
		{
			name: "single request",
			mailbox: []tasks.MailboxMessage{
				{Type: "request", Token: "tok1"},
			},
			want: "tok1",
		},
		{
			name: "request+response — no pending",
			mailbox: []tasks.MailboxMessage{
				{Type: "request", Token: "tok1"},
				{Type: "response", Token: "tok1"},
			},
			want: "",
		},
		{
			name: "two requests, one responded",
			mailbox: []tasks.MailboxMessage{
				{Type: "request", Token: "tok1"},
				{Type: "response", Token: "tok1"},
				{Type: "request", Token: "tok2"},
			},
			want: "tok2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findPendingRequestToken(tt.mailbox)
			if got != tt.want {
				t.Errorf("findPendingRequestToken() = %q, want %q", got, tt.want)
			}
		})
	}
}
