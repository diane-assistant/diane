// Package logger provides structured logging for the Diane server.
// It uses Go's log/slog package with JSON output and file rotation via lumberjack.
package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config holds logger configuration options.
type Config struct {
	// LogDir is the directory where log files are stored.
	// If empty, only stdout logging is enabled.
	LogDir string

	// Debug enables debug-level logging.
	Debug bool

	// JSON enables JSON output format. If false, text format is used.
	JSON bool

	// Component is an optional component name to add to all log entries.
	Component string
}

// Init initializes the global slog logger with the given configuration.
// It writes to both stdout and a rotating log file (if LogDir is specified).
func Init(cfg Config) error {
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}

	var writer io.Writer = os.Stdout

	// Add file logging with rotation if LogDir is specified
	if cfg.LogDir != "" {
		// Ensure log directory exists
		if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
			return err
		}

		logFile := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.LogDir, "server.log"),
			MaxSize:    50,   // megabytes
			MaxBackups: 3,    // number of old files to keep
			MaxAge:     14,   // days
			Compress:   true, // compress rotated files
		}

		// Write to both stdout and file
		writer = io.MultiWriter(os.Stdout, logFile)
	}

	opts := &slog.HandlerOptions{
		Level: level,
		// Add source location for error-level logs
		AddSource: level == slog.LevelDebug,
	}

	var handler slog.Handler
	if cfg.JSON {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	// Add component attribute if specified
	logger := slog.New(handler)
	if cfg.Component != "" {
		logger = logger.With("component", cfg.Component)
	}

	slog.SetDefault(logger)
	return nil
}

// With returns a new logger with the given attributes added to all log entries.
// This is useful for adding context like request IDs or server names.
func With(args ...any) *slog.Logger {
	return slog.Default().With(args...)
}

// WithComponent returns a new logger with a component attribute.
func WithComponent(component string) *slog.Logger {
	return slog.Default().With("component", component)
}

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Info logs at info level.
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Warn logs at warning level.
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Error logs at error level.
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

// Fatal logs at error level and exits with status code 1.
func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
