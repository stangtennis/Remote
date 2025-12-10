package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

var (
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
	DebugLogger *log.Logger
	logFile     *os.File
)

// Init initializes the logger with both file and console output
func Init() error {
	// Get executable directory for log file
	exePath, err := os.Executable()
	if err != nil {
		// Fallback to current directory
		exePath = "."
	}
	exeDir := filepath.Dir(exePath)
	
	// Create log file in same directory as executable (simpler, always works)
	logFilePath := filepath.Join(exeDir, "controller.log")
	
	logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer for both file and console
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// Initialize loggers with different prefixes
	InfoLogger = log.New(multiWriter, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(multiWriter, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	DebugLogger = log.New(multiWriter, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)

	InfoLogger.Printf("Logger initialized. Log file: %s", logFilePath)
	return nil
}

// Close closes the log file
func Close() {
	if logFile != nil {
		InfoLogger.Println("Closing logger")
		logFile.Close()
	}
}

// Info logs an informational message
func Info(format string, v ...interface{}) {
	if InfoLogger != nil {
		InfoLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	if ErrorLogger != nil {
		ErrorLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

// Debug logs a debug message
func Debug(format string, v ...interface{}) {
	if DebugLogger != nil {
		DebugLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

// Fatal logs a fatal error and exits
func Fatal(format string, v ...interface{}) {
	if ErrorLogger != nil {
		ErrorLogger.Output(2, fmt.Sprintf("FATAL: "+format, v...))
	}
	Close()
	os.Exit(1)
}

// GetLogPath returns the path to the log file
func GetLogPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "controller.log"
	}
	return filepath.Join(filepath.Dir(exePath), "controller.log")
}

// ReadLog reads the last N lines from the log file
func ReadLog(maxLines int) (string, error) {
	logPath := GetLogPath()
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to read log: %w", err)
	}
	
	content := string(data)
	lines := splitLines(content)
	
	// Return last N lines
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	
	result := ""
	for _, line := range lines {
		result += line + "\n"
	}
	return result, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
