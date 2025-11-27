package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level represents the minimum level to emit.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger wraps the standard logger with file output.
type Logger struct {
	*log.Logger
	file *os.File
	mu   sync.Mutex
}

var (
	defaultLogger *Logger
	once          sync.Once
	currentLevel  = LevelError
)

// init sets a conservative default before env is read.
func init() {
	currentLevel = chooseLevelFromEnv()
	log.SetOutput(levelWriter{level: LevelInfo})
}

// Initialize sets up the logging system with file output.
func Initialize(logDir string) error {
	var initErr error
	once.Do(func() {
		if err := os.MkdirAll(logDir, 0755); err != nil { //nolint:gosec // Log directory needs group read access
			initErr = fmt.Errorf("failed to create log directory: %w", err)
			return
		}

		logPath := filepath.Join(logDir, "treeos.log")
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644) //nolint:gosec // Log path from config, file needs group read access
		if err != nil {
			initErr = fmt.Errorf("failed to open log file: %w", err)
			return
		}

		multiWriter := io.MultiWriter(os.Stdout, file)

		defaultLogger = &Logger{
			Logger: log.New(multiWriter, "", log.LstdFlags|log.Lshortfile),
			file:   file,
		}

		log.SetOutput(levelWriter{level: LevelInfo})
		log.SetFlags(log.LstdFlags | log.Lshortfile)

		log.Printf("Logging initialized: %s", logPath)
	})
	return initErr
}

// ConfigureLevelFromEnv refreshes the minimum level based on env.
func ConfigureLevelFromEnv() {
	currentLevel = chooseLevelFromEnv()
}

func chooseLevelFromEnv() Level {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		if strings.EqualFold(os.Getenv("DEBUG"), "true") {
			return LevelDebug
		}
		return LevelError
	}
}

// Close closes the log file.
func Close() error {
	if defaultLogger != nil && defaultLogger.file != nil {
		return defaultLogger.file.Close()
	}
	return nil
}

// Debug logs a debug message.
func Debug(v ...interface{}) {
	logAt(LevelDebug, fmt.Sprint(v...))
}

// Debugf logs a formatted debug message.
func Debugf(format string, v ...interface{}) {
	logAt(LevelDebug, fmt.Sprintf(format, v...))
}

// Info logs an info message.
func Info(v ...interface{}) {
	logAt(LevelInfo, fmt.Sprint(v...))
}

// Infof logs a formatted info message.
func Infof(format string, v ...interface{}) {
	logAt(LevelInfo, fmt.Sprintf(format, v...))
}

// Warn logs a warning message.
func Warn(v ...interface{}) {
	logAt(LevelWarn, fmt.Sprint(v...))
}

// Warnf logs a formatted warning message.
func Warnf(format string, v ...interface{}) {
	logAt(LevelWarn, fmt.Sprintf(format, v...))
}

// Error logs an error message.
func Error(v ...interface{}) {
	logAt(LevelError, fmt.Sprint(v...))
}

// Errorf logs a formatted error message.
func Errorf(format string, v ...interface{}) {
	logAt(LevelError, fmt.Sprintf(format, v...))
}

// Fatal logs an error message then exits with code 1.
func Fatal(v ...interface{}) {
	logAt(LevelError, fmt.Sprint(v...))
	os.Exit(1) //nolint:gocritic // Fatal terminates the process intentionally
}

// Fatalf logs a formatted error message then exits with code 1.
func Fatalf(format string, v ...interface{}) {
	logAt(LevelError, fmt.Sprintf(format, v...))
	os.Exit(1) //nolint:gocritic // Fatal terminates the process intentionally
}

// GetLevel exposes the currently configured minimum level.
func GetLevel() Level {
	return currentLevel
}

func logAt(level Level, msg string) {
	if level < currentLevel {
		return
	}

	prefixed := addPrefix(level, msg)

	if defaultLogger != nil {
		defaultLogger.Println(prefixed)
		return
	}

	log.Println(prefixed)
}

func addPrefix(level Level, msg string) string {
	switch level {
	case LevelDebug:
		return "[DEBUG] " + msg
	case LevelInfo:
		return "[INFO] " + msg
	case LevelWarn:
		return "[WARN] " + msg
	default:
		return "[ERROR] " + msg
	}
}

// levelWriter adapts stdlib log output to be level-aware.
type levelWriter struct {
	level Level
}

func (w levelWriter) Write(p []byte) (int, error) {
	if w.level < currentLevel {
		return len(p), nil
	}

	if defaultLogger != nil {
		return defaultLogger.Writer().Write(p)
	}

	return os.Stdout.Write(p)
}

// RotateLogs creates a new log file with timestamp and renames the old one.
func RotateLogs(logDir string) error {
	if defaultLogger == nil {
		return fmt.Errorf("logger not initialized")
	}

	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	if err := defaultLogger.file.Close(); err != nil {
		return fmt.Errorf("failed to close current log file: %w", err)
	}

	oldPath := filepath.Join(logDir, "treeos.log")
	newPath := filepath.Join(logDir, fmt.Sprintf("treeos-%s.log", time.Now().Format("20060102-150405")))
	if err := os.Rename(oldPath, newPath); err != nil {
		file, err := os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644) //nolint:gosec // Log path from config
		if err == nil {
			defaultLogger.file = file
		}
		return fmt.Errorf("failed to rotate log file: %w", err)
	}

	file, err := os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644) //nolint:gosec // Log path from config
	if err != nil {
		return fmt.Errorf("failed to open new log file: %w", err)
	}

	defaultLogger.file = file

	multiWriter := io.MultiWriter(os.Stdout, file)
	defaultLogger.SetOutput(multiWriter)
	log.SetOutput(levelWriter{level: LevelInfo})

	log.Printf("Log rotation completed: %s", newPath)
	return nil
}
