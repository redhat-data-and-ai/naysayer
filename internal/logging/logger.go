package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// Logger provides structured logging with consistent formatting
type Logger struct {
	level  LogLevel
	prefix string
}

// NewLogger creates a new logger with the specified level and prefix
func NewLogger(level LogLevel, prefix string) *Logger {
	return &Logger{
		level:  level,
		prefix: prefix,
	}
}

// GetLogLevel parses a log level string
func GetLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

// formatMessage creates a consistent log message format
func (l *Logger) formatMessage(level string, emoji string, message string, args ...interface{}) string {
	formattedMsg := fmt.Sprintf(message, args...)
	if l.prefix != "" {
		return fmt.Sprintf("[%s] %s %s: %s", l.prefix, emoji, level, formattedMsg)
	}
	return fmt.Sprintf("%s %s: %s", emoji, level, formattedMsg)
}

// Debug logs debug messages
func (l *Logger) Debug(message string, args ...interface{}) {
	if l.level <= DEBUG {
		log.Print(l.formatMessage("DEBUG", "🔍", message, args...))
	}
}

// Info logs info messages
func (l *Logger) Info(message string, args ...interface{}) {
	if l.level <= INFO {
		log.Print(l.formatMessage("INFO", "ℹ️", message, args...))
	}
}

// Warn logs warning messages
func (l *Logger) Warn(message string, args ...interface{}) {
	if l.level <= WARN {
		log.Print(l.formatMessage("WARN", "⚠️", message, args...))
	}
}

// Error logs error messages
func (l *Logger) Error(message string, args ...interface{}) {
	if l.level <= ERROR {
		log.Print(l.formatMessage("ERROR", "❌", message, args...))
	}
}

// Success logs success messages (always shown)
func (l *Logger) Success(message string, args ...interface{}) {
	log.Print(l.formatMessage("SUCCESS", "✅", message, args...))
}

// Webhook specific logging helpers
func (l *Logger) WebhookReceived(projectID, mrIID int, author string) {
	l.Info("Processing webhook: Project=%d, MR=%d, Author=%s", projectID, mrIID, author)
}

func (l *Logger) ApprovalDecision(mrIID int, decision, reason string) {
	if decision == "approve" {
		l.Success("Auto-approved MR %d: %s", mrIID, reason)
	} else {
		l.Warn("Manual review required for MR %d: %s", mrIID, reason)
	}
}

func (l *Logger) APIError(operation string, err error) {
	l.Error("GitLab API %s failed: %v", operation, err)
}

func (l *Logger) RuleEvaluation(ruleName string, decision, reason string) {
	emoji := "✅"
	if decision != "approve" {
		emoji = "🚫"
	}
	l.Info("%s Rule %s: %s", emoji, ruleName, reason)
}

// Global logger instance
var defaultLogger *Logger

// InitLogger initializes the global logger
func InitLogger(level string, component string) {
	logLevel := GetLogLevel(level)
	defaultLogger = NewLogger(logLevel, component)
}

// Global logging functions
func Debug(message string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Debug(message, args...)
	} else {
		log.Printf("DEBUG: "+message, args...)
	}
}

func Info(message string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Info(message, args...)
	} else {
		log.Printf("INFO: "+message, args...)
	}
}

func Warn(message string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Warn(message, args...)
	} else {
		log.Printf("WARN: "+message, args...)
	}
}

func Error(message string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Error(message, args...)
	} else {
		log.Printf("ERROR: "+message, args...)
	}
}

func Success(message string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Success(message, args...)
	} else {
		log.Printf("SUCCESS: "+message, args...)
	}
}

// Webhook-specific global helpers
func WebhookReceived(projectID, mrIID int, author string) {
	if defaultLogger != nil {
		defaultLogger.WebhookReceived(projectID, mrIID, author)
	} else {
		log.Printf("Processing webhook: Project=%d, MR=%d, Author=%s", projectID, mrIID, author)
	}
}

func ApprovalDecision(mrIID int, decision, reason string) {
	if defaultLogger != nil {
		defaultLogger.ApprovalDecision(mrIID, decision, reason)
	} else {
		log.Printf("Decision for MR %d: %s - %s", mrIID, decision, reason)
	}
}

func APIError(operation string, err error) {
	if defaultLogger != nil {
		defaultLogger.APIError(operation, err)
	} else {
		log.Printf("API Error in %s: %v", operation, err)
	}
}

func RuleEvaluation(ruleName string, decision, reason string) {
	if defaultLogger != nil {
		defaultLogger.RuleEvaluation(ruleName, decision, reason)
	} else {
		log.Printf("Rule %s: %s - %s", ruleName, decision, reason)
	}
}

func init() {
	// Set default log flags
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	// Initialize with default logger if not already done
	if defaultLogger == nil {
		level := os.Getenv("LOG_LEVEL")
		if level == "" {
			level = "info"
		}
		InitLogger(level, "NAYSAYER")
	}
} 
