// Package logging provides structured logging utilities
package logging

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// LogLevel represents log severity
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// Logger provides structured logging
type Logger struct {
	service string
	level   LogLevel
}

// NewLogger creates a new logger
func NewLogger(service string) *Logger {
	return &Logger{
		service: service,
		level:   LogLevelInfo,
	}
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// Fields represents structured log fields
type Fields map[string]any

// Debug logs a debug message
func (l *Logger) Debug(ctx context.Context, msg string, fields Fields) {
	if l.level <= LogLevelDebug {
		l.log("DEBUG", msg, fields)
	}
}

// Info logs an info message
func (l *Logger) Info(ctx context.Context, msg string, fields Fields) {
	if l.level <= LogLevelInfo {
		l.log("INFO", msg, fields)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(ctx context.Context, msg string, fields Fields) {
	if l.level <= LogLevelWarn {
		l.log("WARN", msg, fields)
	}
}

// Error logs an error message
func (l *Logger) Error(ctx context.Context, msg string, err error, fields Fields) {
	if l.level <= LogLevelError {
		if fields == nil {
			fields = Fields{}
		}
		if err != nil {
			fields["error"] = err.Error()
		}
		l.log("ERROR", msg, fields)
	}
}

// log outputs the log message
func (l *Logger) log(level, msg string, fields Fields) {
	timestamp := time.Now().Format(time.RFC3339)

	var output strings.Builder
	output.WriteString(timestamp)
	output.WriteString(" [")
	output.WriteString(level)
	output.WriteString("] ")
	output.WriteString(l.service)
	output.WriteString(": ")
	output.WriteString(msg)

	if len(fields) > 0 {
		output.WriteString(" ")
		for k, v := range fields {
			output.WriteString(k)
			output.WriteString("=")
			output.WriteString(formatValue(v))
			output.WriteString(" ")
		}
	}

	log.Println(output.String())
}

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%.2f", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// Default logger instance
var defaultLogger = NewLogger("uta")

// SetDefaultLogger sets the default logger
func SetDefaultLogger(logger *Logger) {
	defaultLogger = logger
}

// GetLogger returns the default logger
func GetLogger() *Logger {
	return defaultLogger
}

// Debug logs using default logger
func Debug(ctx context.Context, msg string, fields Fields) {
	defaultLogger.Debug(ctx, msg, fields)
}

// Info logs using default logger
func Info(ctx context.Context, msg string, fields Fields) {
	defaultLogger.Info(ctx, msg, fields)
}

// Warn logs using default logger
func Warn(ctx context.Context, msg string, fields Fields) {
	defaultLogger.Warn(ctx, msg, fields)
}

// Error logs using default logger
func Error(ctx context.Context, msg string, err error, fields Fields) {
	defaultLogger.Error(ctx, msg, err, fields)
}

// Init initializes logging
func Init(service string, level string) {
	defaultLogger = NewLogger(service)

	switch level {
	case "debug":
		defaultLogger.SetLevel(LogLevelDebug)
	case "warn":
		defaultLogger.SetLevel(LogLevelWarn)
	case "error":
		defaultLogger.SetLevel(LogLevelError)
	default:
		defaultLogger.SetLevel(LogLevelInfo)
	}

	log.SetOutput(os.Stdout)
	log.SetFlags(0) // We handle timestamp ourselves
}