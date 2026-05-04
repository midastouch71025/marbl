package logging

import (
	"log/slog"
	"os"
	"strings"
)

func New(level, format string) *slog.Logger {
	lvl := parseLevel(level)
	var h slog.Handler
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	default:
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	}
	return slog.New(h)
}

func parseLevel(s string) slog.Leveler {
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
