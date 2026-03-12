package introspection

import (
	"log/slog"
	"testing"
)

func TestResolveLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
		{" debug ", slog.LevelDebug},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ResolveLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("ResolveLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
