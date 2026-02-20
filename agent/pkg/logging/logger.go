package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Logger      zerolog.Logger
	logFilePath string              // Path to log file for syncing
	fileWriter  *lumberjack.Logger  // Reference for Sync (Close flushes to disk)
)

// Config holds logging configuration
type Config struct {
	Level      string // debug, info, warn, error
	LogDir     string // Directory for log files
	MaxSize    int    // Max size in MB before rotation
	MaxBackups int    // Max number of old log files to keep
	MaxAge     int    // Max age in days to keep old log files
	Compress   bool   // Compress rotated files
	Console    bool   // Also log to console
}

// DefaultConfig returns default logging configuration
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		LogDir:     getDefaultLogDir(),
		MaxSize:    10,  // 10 MB
		MaxBackups: 5,   // Keep 5 old files
		MaxAge:     30,  // Keep for 30 days
		Compress:   true,
		Console:    true,
	}
}

// Init initializes the logger with the given configuration
func Init(cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure log rotation
	logFilePath = filepath.Join(cfg.LogDir, "agent.log")
	fileWriter = &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	// Setup writers
	var writers []io.Writer
	writers = append(writers, fileWriter)

	if cfg.Console {
		// Console writer with pretty formatting
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
		writers = append(writers, consoleWriter)
	}

	// Create multi-writer
	multi := io.MultiWriter(writers...)

	// Redirect standard log package output to same writers
	// This ensures log.Println() calls (used in startAgent etc.) are captured
	log.SetOutput(multi)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Initialize logger
	Logger = zerolog.New(multi).With().
		Timestamp().
		Caller().
		Logger()

	Logger.Info().
		Str("level", cfg.Level).
		Str("log_dir", cfg.LogDir).
		Int("max_size_mb", cfg.MaxSize).
		Int("max_backups", cfg.MaxBackups).
		Msg("Logger initialized")

	return nil
}

// getDefaultLogDir returns the default log directory based on OS
func getDefaultLogDir() string {
	// macOS: ~/Library/Logs/RemoteDesktopAgent/
	if home, err := os.UserHomeDir(); err == nil {
		macLogDir := filepath.Join(home, "Library", "Logs", "RemoteDesktopAgent")
		if _, err := os.Stat(filepath.Join(home, "Library")); err == nil {
			return macLogDir
		}
	}

	// Windows: %APPDATA%/RemoteAgent/logs/
	if os.Getenv("APPDATA") != "" {
		return filepath.Join(os.Getenv("APPDATA"), "RemoteAgent", "logs")
	}

	// Linux/other Unix
	home, err := os.UserHomeDir()
	if err != nil {
		return "./logs"
	}
	return filepath.Join(home, ".remote-agent", "logs")
}

// Sync flushes the log file to disk by closing and reopening lumberjack's file handle.
// lumberjack.Close() flushes pending data and closes the file.
// The next Write() call automatically reopens it.
// This is thread-safe (lumberjack uses internal mutex).
func Sync() {
	if fileWriter != nil {
		fileWriter.Close()
	}
}

// GetLogFilePath returns the path to the current log file
func GetLogFilePath() string {
	return logFilePath
}

// Info logs an info message
func Info(msg string) {
	Logger.Info().Msg(msg)
}

// Debug logs a debug message
func Debug(msg string) {
	Logger.Debug().Msg(msg)
}

// Warn logs a warning message
func Warn(msg string) {
	Logger.Warn().Msg(msg)
}

// Error logs an error message
func Error(err error, msg string) {
	Logger.Error().Err(err).Msg(msg)
}

// Fatal logs a fatal message and exits
func Fatal(err error, msg string) {
	Logger.Fatal().Err(err).Msg(msg)
}

// WithFields returns a logger with additional fields
func WithFields(fields map[string]interface{}) *zerolog.Logger {
	ctx := Logger.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	logger := ctx.Logger()
	return &logger
}
