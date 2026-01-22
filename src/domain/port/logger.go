package port

import "time"

// Field represents a structured logging field.
type Field struct {
	Key   string
	Value any
}

// String returns a string field.
func String(key string, value string) Field {
	return Field{Key: key, Value: value}
}

// Int returns an int field.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 returns an int64 field.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Duration returns a duration field.
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value.Milliseconds()}
}

// Bool returns a bool field.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Error returns an error field.
func Error(err error) Field {
	return Field{Key: "error", Value: err.Error()}
}

// Logger interface for structured logging.
// This interface allows dependency injection of logging for better testability.
type Logger interface {
	// Debug logs a debug message.
	Debug(msg string, fields ...Field)
	// Info logs an info message.
	Info(msg string, fields ...Field)
	// Warn logs a warning message.
	Warn(msg string, fields ...Field)
	// Error logs an error message.
	Error(msg string, fields ...Field)
	// Fatal logs a fatal message and exits.
	Fatal(msg string, fields ...Field)
	// With returns a new logger with the given fields.
	With(fields ...Field) Logger
}

// RequestLogger interface for request-specific logging.
type RequestLogger interface {
	// LogRequest logs a complete request.
	LogRequest(reqID string, content string)
	// LogError logs an error with request context.
	LogError(reqID string, content string)
}

// NopLogger is a no-op logger implementation for testing.
type NopLogger struct{}

func (n *NopLogger) Debug(msg string, fields ...Field)               {}
func (n *NopLogger) Info(msg string, fields ...Field)                {}
func (n *NopLogger) Warn(msg string, fields ...Field)                {}
func (n *NopLogger) Error(msg string, fields ...Field)               {}
func (n *NopLogger) Fatal(msg string, fields ...Field)               {}
func (n *NopLogger) With(fields ...Field) Logger                     { return n }
func (n *NopLogger) LogRequest(reqID string, content string)         {}
func (n *NopLogger) LogError(reqID string, content string)           {}
