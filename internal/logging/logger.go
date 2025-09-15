// Package logging provides unified logging infrastructure for TreeOS
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger wraps the standard logger with file output
type Logger struct {
	*log.Logger
	file *os.File
	mu   sync.Mutex
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// Initialize sets up the logging system with file output
func Initialize(logDir string) error {
	var initErr error
	once.Do(func() {
		// Create logs directory
		if err := os.MkdirAll(logDir, 0755); err != nil {
			initErr = fmt.Errorf("failed to create log directory: %w", err)
			return
		}

		// Open log file with rotation-friendly naming
		logPath := filepath.Join(logDir, "treeos.log")
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			initErr = fmt.Errorf("failed to open log file: %w", err)
			return
		}

		// Create multi-writer for stdout and file
		multiWriter := io.MultiWriter(os.Stdout, file)

		defaultLogger = &Logger{
			Logger: log.New(multiWriter, "", log.LstdFlags|log.Lshortfile),
			file:   file,
		}

		// Replace default logger
		log.SetOutput(multiWriter)
		log.SetFlags(log.LstdFlags | log.Lshortfile)

		// Log initialization
		log.Printf("Logging initialized: %s", logPath)
	})
	return initErr
}

// Close closes the log file
func Close() error {
	if defaultLogger != nil && defaultLogger.file != nil {
		return defaultLogger.file.Close()
	}
	return nil
}

// Printf logs a formatted message
func Printf(format string, v ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Printf(format, v...)
	} else {
		log.Printf(format, v...)
	}
}

// Println logs a message with newline
func Println(v ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Println(v...)
	} else {
		log.Println(v...)
	}
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	msg := fmt.Sprintf("[ERROR] "+format, v...)
	if defaultLogger != nil {
		defaultLogger.Println(msg)
	} else {
		log.Println(msg)
	}
}

// Warning logs a warning message
func Warning(format string, v ...interface{}) {
	msg := fmt.Sprintf("[WARN] "+format, v...)
	if defaultLogger != nil {
		defaultLogger.Println(msg)
	} else {
		log.Println(msg)
	}
}

// Info logs an info message
func Info(format string, v ...interface{}) {
	msg := fmt.Sprintf("[INFO] "+format, v...)
	if defaultLogger != nil {
		defaultLogger.Println(msg)
	} else {
		log.Println(msg)
	}
}

// Debug logs a debug message (only in development mode)
func Debug(format string, v ...interface{}) {
	if os.Getenv("DEBUG") == "true" {
		msg := fmt.Sprintf("[DEBUG] "+format, v...)
		if defaultLogger != nil {
			defaultLogger.Println(msg)
		} else {
			log.Println(msg)
		}
	}
}

// RotateLogs creates a new log file with timestamp and renames the old one
func RotateLogs(logDir string) error {
	if defaultLogger == nil {
		return fmt.Errorf("logger not initialized")
	}

	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	// Close current file
	if err := defaultLogger.file.Close(); err != nil {
		return fmt.Errorf("failed to close current log file: %w", err)
	}

	// Rename current log file with timestamp
	oldPath := filepath.Join(logDir, "treeos.log")
	newPath := filepath.Join(logDir, fmt.Sprintf("treeos-%s.log", time.Now().Format("20060102-150405")))
	if err := os.Rename(oldPath, newPath); err != nil {
		// If rename fails, try to reopen the original file
		defaultLogger.file, _ = os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		return fmt.Errorf("failed to rotate log file: %w", err)
	}

	// Open new log file
	file, err := os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open new log file: %w", err)
	}

	defaultLogger.file = file

	// Update multi-writer
	multiWriter := io.MultiWriter(os.Stdout, file)
	defaultLogger.Logger.SetOutput(multiWriter)
	log.SetOutput(multiWriter)

	log.Printf("Log rotation completed: %s", newPath)
	return nil
}