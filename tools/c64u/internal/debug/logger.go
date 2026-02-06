package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var (
	enabled  bool
	logFile  *os.File
	logPath  string
)

// Init initializes the debug logger
// If debug is true, creates log directory and opens log file
func Init(debug bool) error {
	enabled = debug
	if !enabled {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	logDir := filepath.Join(homeDir, ".local", "share", "c64u")
	logPath = filepath.Join(logDir, "c64u.log")

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}

	// Open log file (overwrite on each start)
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}

	Log("Debug logging initialized")
	Log("Log file: %s", logPath)

	return nil
}

// Close closes the log file
func Close() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// IsEnabled returns true if debug logging is enabled
func IsEnabled() bool {
	return enabled
}

// GetLogPath returns the path to the log file
func GetLogPath() string {
	return logPath
}

// Log writes a debug message to both stderr and log file
func Log(format string, args ...interface{}) {
	if !enabled {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] DEBUG: %s\n", timestamp, message)

	// Write to stderr
	fmt.Fprint(os.Stderr, logLine)

	// Write to log file
	if logFile != nil {
		logFile.WriteString(logLine)
	}
}

// LogError writes an error-level debug message
func LogError(format string, args ...interface{}) {
	if !enabled {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("[%s] ERROR: %s\n", timestamp, message)

	// Write to stderr
	fmt.Fprint(os.Stderr, logLine)

	// Write to log file
	if logFile != nil {
		logFile.WriteString(logLine)
	}
}
