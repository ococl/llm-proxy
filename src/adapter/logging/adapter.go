package logging

import (
	"time"

	"go.uber.org/zap"

	"llm-proxy/domain/port"
)

// ZapLoggerAdapter adapts zap.SugaredLogger to port.Logger interface.
type ZapLoggerAdapter struct {
	sugar *zap.SugaredLogger
}

// NewZapLoggerAdapter creates a new ZapLoggerAdapter.
func NewZapLoggerAdapter(sugar *zap.SugaredLogger) *ZapLoggerAdapter {
	return &ZapLoggerAdapter{
		sugar: sugar,
	}
}

// Debug logs a debug message.
func (z *ZapLoggerAdapter) Debug(msg string, fields ...port.Field) {
	if len(fields) == 0 {
		z.sugar.Debug(msg)
		return
	}
	z.sugar.Debugw(msg, toInterfacePairs(fields)...)
}

// Info logs an info message.
func (z *ZapLoggerAdapter) Info(msg string, fields ...port.Field) {
	if len(fields) == 0 {
		z.sugar.Info(msg)
		return
	}
	z.sugar.Infow(msg, toInterfacePairs(fields)...)
}

// Warn logs a warning message.
func (z *ZapLoggerAdapter) Warn(msg string, fields ...port.Field) {
	if len(fields) == 0 {
		z.sugar.Warn(msg)
		return
	}
	z.sugar.Warnw(msg, toInterfacePairs(fields)...)
}

// Error logs an error message.
func (z *ZapLoggerAdapter) Error(msg string, fields ...port.Field) {
	if len(fields) == 0 {
		z.sugar.Error(msg)
		return
	}
	z.sugar.Errorw(msg, toInterfacePairs(fields)...)
}

// Fatal logs a fatal message and exits.
func (z *ZapLoggerAdapter) Fatal(msg string, fields ...port.Field) {
	if len(fields) == 0 {
		z.sugar.Fatal(msg)
		return
	}
	z.sugar.Fatalw(msg, toInterfacePairs(fields)...)
}

// With returns a new logger with the given fields.
func (z *ZapLoggerAdapter) With(fields ...port.Field) port.Logger {
	if len(fields) == 0 {
		return z
	}
	newSugar := z.sugar.With(toInterfacePairs(fields)...)
	return NewZapLoggerAdapter(newSugar)
}

// LogRequest logs a complete request.
func (z *ZapLoggerAdapter) LogRequest(reqID string, content string) {
	z.sugar.Infow("request_log",
		"trace_id", reqID,
		"content", content,
	)
}

// LogError logs an error with request context.
func (z *ZapLoggerAdapter) LogError(reqID string, content string) {
	z.sugar.Errorw("error_log",
		"trace_id", reqID,
		"content", content,
	)
}

// toInterfacePairs converts port.Field slice to interface{} pairs for SugaredLogger.
func toInterfacePairs(fields []port.Field) []interface{} {
	if len(fields) == 0 {
		return nil
	}

	pairs := make([]interface{}, 0, len(fields)*2)
	for _, f := range fields {
		pairs = append(pairs, f.Key, convertValue(f.Value))
	}
	return pairs
}

// convertValue converts field value to appropriate type for SugaredLogger.
func convertValue(value any) any {
	switch v := value.(type) {
	case time.Duration:
		return v.Milliseconds()
	case error:
		if v != nil {
			return v.Error()
		}
		return ""
	default:
		return v
	}
}
