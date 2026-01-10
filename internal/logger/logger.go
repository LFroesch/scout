package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logFile *os.File
	mu      sync.Mutex
	enabled = true
)

const (
	maxLogSize = 5 * 1024 * 1024 // 5MB
)

// Init initializes the logger and creates the log file
func Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot get home directory: %w", err)
	}

	logDir := filepath.Join(homeDir, ".config", "scout")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("cannot create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, "scout.log")

	// Check if log file needs rotation
	if info, err := os.Stat(logPath); err == nil {
		if info.Size() > maxLogSize {
			// Rotate log by renaming to .old
			oldPath := logPath + ".old"
			os.Remove(oldPath) // Remove old backup if exists
			os.Rename(logPath, oldPath)
		}
	}

	// Open or create log file
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("cannot open log file: %w", err)
	}

	logFile = file
	return nil
}

// Close closes the log file
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// Disable disables logging (useful for tests)
func Disable() {
	mu.Lock()
	defer mu.Unlock()
	enabled = false
}

// Enable enables logging
func Enable() {
	mu.Lock()
	defer mu.Unlock()
	enabled = true
}

// Error logs an error message
func Error(format string, args ...any) {
	log("ERROR", format, args...)
}

// Warn logs a warning message
func Warn(format string, args ...any) {
	log("WARN", format, args...)
}

// log writes a log message to the file
func log(level string, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()

	if !enabled || logFile == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] %s: %s\n", timestamp, level, message)

	logFile.WriteString(logLine)
}
