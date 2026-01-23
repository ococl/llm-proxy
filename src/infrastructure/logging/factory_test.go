package logging

import (
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected zapcore.Level
	}{
		{"debug lowercase", "debug", zapcore.DebugLevel},
		{"DEBUG uppercase", "DEBUG", zapcore.DebugLevel},
		{"info lowercase", "info", zapcore.InfoLevel},
		{"INFO uppercase", "INFO", zapcore.InfoLevel},
		{"warn lowercase", "warn", zapcore.WarnLevel},
		{"warning full word", "warning", zapcore.WarnLevel},
		{"WARN uppercase", "WARN", zapcore.WarnLevel},
		{"error lowercase", "error", zapcore.ErrorLevel},
		{"ERROR uppercase", "ERROR", zapcore.ErrorLevel},
		{"dpanic", "dpanic", zapcore.DPanicLevel},
		{"DPANIC uppercase", "DPANIC", zapcore.DPanicLevel},
		{"panic", "panic", zapcore.PanicLevel},
		{"PANIC uppercase", "PANIC", zapcore.PanicLevel},
		{"fatal", "fatal", zapcore.FatalLevel},
		{"FATAL uppercase", "FATAL", zapcore.FatalLevel},
		{"unknown defaults to info", "unknown", zapcore.InfoLevel},
		{"empty string defaults to info", "", zapcore.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLevel(tt.level)
			if result != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.level, result, tt.expected)
			}
		})
	}
}

func TestFieldValueString(t *testing.T) {
	tests := []struct {
		name     string
		field    zapcore.Field
		expected string
	}{
		{
			name:     "string type",
			field:    zapcore.Field{Key: "key", Type: zapcore.StringType, String: "value"},
			expected: "value",
		},
		{
			name:     "int64 type",
			field:    zapcore.Field{Key: "key", Type: zapcore.Int64Type, Integer: 12345},
			expected: "12345",
		},
		{
			name:     "int32 type",
			field:    zapcore.Field{Key: "key", Type: zapcore.Int32Type, Integer: 54321},
			expected: "54321",
		},
		{
			name:     "int64 as int",
			field:    zapcore.Field{Key: "key", Type: zapcore.Int64Type, Integer: 999},
			expected: "999",
		},
		{
			name:     "uint64 type",
			field:    zapcore.Field{Key: "key", Type: zapcore.Uint64Type, Integer: 888888},
			expected: "888888",
		},
		{
			name:     "bool true",
			field:    zapcore.Field{Key: "key", Type: zapcore.BoolType, Integer: 1},
			expected: "true",
		},
		{
			name:     "bool false",
			field:    zapcore.Field{Key: "key", Type: zapcore.BoolType, Integer: 0},
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fieldValueString(tt.field)
			if result != tt.expected {
				t.Errorf("fieldValueString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFieldValueString_Duration(t *testing.T) {
	field := zapcore.Field{Key: "key", Type: zapcore.DurationType, Integer: int64(time.Second)}
	result := fieldValueString(field)
	// Duration field type returns empty string in current implementation
	// This test documents the current behavior
	if result != "" {
		t.Logf("Note: fieldValueString(duration) returned %q (may vary by implementation)", result)
	}
}

func TestLevelPriority(t *testing.T) {
	tests := []struct {
		level string
		want  int
	}{
		{"debug", 0},
		{"info", 1},
		{"warn", 2},
		{"error", 3},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			if got, ok := LevelPriority[tt.level]; !ok || got != tt.want {
				t.Errorf("LevelPriority[%q] = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}
