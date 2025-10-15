package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	currentLevel = INFO
	logger       = log.New(os.Stdout, "", 0)
	errorLogger  = log.New(os.Stderr, "", 0)
	logFile      *os.File
)

func init() {
	// Check environment for log level
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		switch level {
		case "DEBUG":
			currentLevel = DEBUG
		case "INFO":
			currentLevel = INFO
		case "WARN":
			currentLevel = WARN
		case "ERROR":
			currentLevel = ERROR
		}
	}
}

// InitializeFileLogging sets up logging to a file in addition to stdout
func InitializeFileLogging(logDir string) error {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file
	logPath := filepath.Join(logDir, "server.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	logFile = file

	// Write to both stdout and file
	multiWriter := io.MultiWriter(os.Stdout, file)
	logger = log.New(multiWriter, "", 0)

	// Errors to both stderr and file
	errorMultiWriter := io.MultiWriter(os.Stderr, file)
	errorLogger = log.New(errorMultiWriter, "", 0)

	Info("📝 Logging initialized - writing to: %s", logPath)
	return nil
}

// CloseLogFile closes the log file (call on shutdown)
func CloseLogFile() {
	if logFile != nil {
		logFile.Close()
	}
}

func timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05.000")
}

func Debug(format string, v ...interface{}) {
	if currentLevel <= DEBUG {
		msg := fmt.Sprintf(format, v...)
		logger.Printf("[%s] DEBUG: %s", timestamp(), msg)
	}
}

func Info(format string, v ...interface{}) {
	if currentLevel <= INFO {
		msg := fmt.Sprintf(format, v...)
		logger.Printf("[%s] INFO:  %s", timestamp(), msg)
	}
}

func Warn(format string, v ...interface{}) {
	if currentLevel <= WARN {
		msg := fmt.Sprintf(format, v...)
		logger.Printf("[%s] WARN:  %s", timestamp(), msg)
	}
}

func Error(format string, v ...interface{}) {
	if currentLevel <= ERROR {
		msg := fmt.Sprintf(format, v...)
		errorLogger.Printf("[%s] ERROR: %s", timestamp(), msg)
	}
}

// Request logging
func Request(method, path, ip string, duration time.Duration, status int) {
	statusColor := ""
	if status >= 500 {
		statusColor = "🔴"
	} else if status >= 400 {
		statusColor = "🟡"
	} else if status >= 300 {
		statusColor = "🔵"
	} else {
		statusColor = "🟢"
	}

	Info("%s %s %s %s - %d (%v)", statusColor, method, path, ip, status, duration)
}

// SQL query logging
func Query(query string, args []interface{}, duration time.Duration, err error) {
	if err != nil {
		Error("SQL Error: %v | Query: %s | Args: %v | Duration: %v", err, query, args, duration)
	} else if currentLevel == DEBUG {
		Debug("SQL: %s | Args: %v | Duration: %v", query, args, duration)
	}
}

// Handler error logging with context
func HandlerError(handler, endpoint string, err error, statusCode int) {
	Error("[%s] %s - Status %d: %v", handler, endpoint, statusCode, err)
}
