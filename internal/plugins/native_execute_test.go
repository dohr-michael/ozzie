package plugins

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestExecuteTool_BasicCommand(t *testing.T) {
	tool := NewExecuteTool()
	result, err := tool.InvokableRun(context.Background(), `{"command": "echo hello"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out executeOutput
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !strings.Contains(out.Stdout, "hello") {
		t.Errorf("expected stdout to contain 'hello', got %q", out.Stdout)
	}
	if out.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", out.ExitCode)
	}
}

func TestExecuteTool_EmptyCommand(t *testing.T) {
	tool := NewExecuteTool()
	_, err := tool.InvokableRun(context.Background(), `{"command": ""}`)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestExecuteTool_NonZeroExit(t *testing.T) {
	tool := NewExecuteTool()
	result, err := tool.InvokableRun(context.Background(), `{"command": "exit 42"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out executeOutput
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", out.ExitCode)
	}
}

func TestExecuteTool_Timeout(t *testing.T) {
	tool := NewExecuteTool()
	_, err := tool.InvokableRun(context.Background(), `{"command": "sleep 10", "timeout": 1}`)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestExecuteTool_Info(t *testing.T) {
	tool := NewExecuteTool()
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "run_command" {
		t.Errorf("expected name 'run_command', got %q", info.Name)
	}
}

func TestIsSudo(t *testing.T) {
	tests := []struct {
		name string
		json string
		want bool
	}{
		{"sudo true", `{"sudo": true}`, true},
		{"sudo false", `{"sudo": false}`, false},
		{"no sudo field", `{"command": "ls"}`, false},
		{"invalid json", `{bad`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSudo(tt.json); got != tt.want {
				t.Errorf("IsSudo(%s) = %v, want %v", tt.json, got, tt.want)
			}
		})
	}
}
