package introspection

import (
	"log/slog"
	"os"
	"strings"
)

// SetupLogger configures the global slog logger with the given level.
func SetupLogger(level slog.Level) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

// ResolveLogLevel maps a config string ("debug", "info", "warn", "error") to slog.Level.
func ResolveLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
