package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetup_LevelParsing(t *testing.T) {
	tests := []struct {
		level    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tc := range tests {
		t.Run(tc.level, func(t *testing.T) {
			closeFn, err := Setup(tc.level, "")
			if err != nil {
				t.Fatalf("Setup(%q): %v", tc.level, err)
			}
			defer closeFn()

			handler := slog.Default().Handler()
			if !handler.Enabled(nil, tc.expected) {
				t.Errorf("expected level %v to be enabled for %q", tc.expected, tc.level)
			}
		})
	}
}

func TestSetup_FileOutput(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "test.log")

	closeFn, err := Setup("info", logPath)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closeFn()

	slog.Info("test message")

	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("expected log file at %s: %v", logPath, err)
	}
}

func TestSetup_DebugFilteredAtInfoLevel(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "filtered.log")

	closeFn, err := Setup("info", logPath)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer closeFn()

	slog.Debug("this should not appear")
	slog.Info("this should appear")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "should not appear") {
		t.Error("debug message should be filtered at info level")
	}
	if !strings.Contains(content, "should appear") {
		t.Error("info message should appear at info level")
	}
}

func TestSetup_StderrNoCloseError(t *testing.T) {
	closeFn, err := Setup("info", "")
	if err != nil {
		t.Fatalf("Setup with empty file: %v", err)
	}
	// Close should not panic or error for stderr mode
	closeFn()
	closeFn() // double close should also be safe
}
