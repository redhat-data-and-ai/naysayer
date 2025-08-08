package logging

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// Logger wraps zap.Logger to provide a consistent interface
type Logger struct {
	zap   *zap.Logger
	level LogLevel
}

// NewLogger creates a new Zap-based logger
func NewLogger(level LogLevel, component string) *Logger {
	// Configure Zap
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(logLevelToZap(level))
	config.Development = false
	config.Encoding = "json"
	
	// Set initial fields
	config.InitialFields = map[string]interface{}{
		"component": component,
		"service":   "naysayer",
	}

	// Build the logger
	zapLogger, err := config.Build()
	if err != nil {
		// Fallback to development logger if production config fails
		zapLogger, _ = zap.NewDevelopment()
	}

	return &Logger{
		zap:   zapLogger,
		level: level,
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

// logLevelToZap converts our LogLevel to zap level
func logLevelToZap(level LogLevel) zapcore.Level {
	switch level {
	case DEBUG:
		return zapcore.DebugLevel
	case INFO:
		return zapcore.InfoLevel
	case WARN:
		return zapcore.WarnLevel
	case ERROR:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// Info logs info messages
func (l *Logger) Info(message string, args ...interface{}) {
	if len(args) == 0 {
		l.zap.Info(message)
	} else {
		l.zap.Sugar().Infof(message, args...)
	}
}

// Warn logs warning messages
func (l *Logger) Warn(message string, args ...interface{}) {
	if len(args) == 0 {
		l.zap.Warn(message)
	} else {
		l.zap.Sugar().Warnf(message, args...)
	}
}

// Error logs error messages
func (l *Logger) Error(message string, args ...interface{}) {
	if len(args) == 0 {
		l.zap.Error(message)
	} else {
		l.zap.Sugar().Errorf(message, args...)
	}
}

// MR-specific logging helpers for better traceability
func (l *Logger) MRInfo(mrID int, message string, fields ...zap.Field) {
	allFields := append([]zap.Field{zap.Int("mr_id", mrID)}, fields...)
	l.zap.Info(message, allFields...)
}

func (l *Logger) MRError(mrID int, message string, err error, fields ...zap.Field) {
	allFields := append([]zap.Field{
		zap.Int("mr_id", mrID),
		zap.Error(err),
	}, fields...)
	l.zap.Error(message, allFields...)
}

func (l *Logger) MRWarn(mrID int, message string, fields ...zap.Field) {
	allFields := append([]zap.Field{zap.Int("mr_id", mrID)}, fields...)
	l.zap.Warn(message, allFields...)
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() {
	l.zap.Sync()
}

// Global logger instance
var defaultLogger *Logger

// InitLogger initializes the global logger
func InitLogger(level string, component string) {
	logLevel := GetLogLevel(level)
	defaultLogger = NewLogger(logLevel, component)
}

// Global logging functions (only the ones actually used)
func Info(message string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Info(message, args...)
	}
}

func Warn(message string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Warn(message, args...)
	}
}

func Error(message string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Error(message, args...)
	}
}

// MR-specific global helpers
func MRInfo(mrID int, message string, fields ...zap.Field) {
	if defaultLogger != nil {
		defaultLogger.MRInfo(mrID, message, fields...)
	}
}

func MRError(mrID int, message string, err error, fields ...zap.Field) {
	if defaultLogger != nil {
		defaultLogger.MRError(mrID, message, err, fields...)
	}
}

func MRWarn(mrID int, message string, fields ...zap.Field) {
	if defaultLogger != nil {
		defaultLogger.MRWarn(mrID, message, fields...)
	}
}

// GetLogger returns the default logger instance
func GetLogger() *Logger {
	return defaultLogger
}

func init() {
	// Initialize with default logger if not already done
	if defaultLogger == nil {
		level := os.Getenv("LOG_LEVEL")
		if level == "" {
			level = "info"
		}
		InitLogger(level, "NAYSAYER")
	}
} 
