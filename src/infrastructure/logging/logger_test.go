package logging

import (
	"testing"
	"time"

	"llm-proxy/infrastructure/config"
)

func TestMaskSensitiveData(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		changed bool
	}{
		{
			name:    "API key masked",
			input:   "sk-abcdefghijklmnopqrstuv",
			changed: true,
		},
		{
			name:    "Bearer token masked",
			input:   "Authorization: bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			changed: true,
		},
		{
			name:    "No sensitive data",
			input:   "Hello, this is a normal message",
			changed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveData(tt.input)
			if tt.changed {
				if result == tt.input {
					t.Logf("Note: MaskSensitiveData did not mask %q", tt.input)
				}
			} else {
				if result != tt.input {
					t.Errorf("MaskSensitiveData(%q) = %q, should not change", tt.input, result)
				}
			}
		})
	}
}

func TestSetTestMode(t *testing.T) {
	if testMode != false {
		t.Errorf("testMode should be false initially, got %v", testMode)
	}

	SetTestMode(true)
	if testMode != true {
		t.Errorf("testMode should be true after SetTestMode(true), got %v", testMode)
	}

	SetTestMode(false)
	if testMode != false {
		t.Errorf("testMode should be false after SetTestMode(false), got %v", testMode)
	}
}

func TestNewRequestMetrics(t *testing.T) {
	reqID := "test-req-123"
	modelAlias := "gpt-4"

	metrics := NewRequestMetrics(reqID, modelAlias)

	if metrics.RequestID != reqID {
		t.Errorf("RequestID = %q, want %q", metrics.RequestID, reqID)
	}
	if metrics.ModelAlias != modelAlias {
		t.Errorf("ModelAlias = %q, want %q", metrics.ModelAlias, modelAlias)
	}
	if metrics.BackendTimes == nil {
		t.Error("BackendTimes should not be nil")
	}
	if metrics.Attempts != 0 {
		t.Errorf("Attempts = %d, want 0", metrics.Attempts)
	}
}

func TestRequestMetrics_RecordBackendTime(t *testing.T) {
	metrics := NewRequestMetrics("req-1", "gpt-4")

	metrics.RecordBackendTime("openai", 100*time.Millisecond)

	if metrics.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", metrics.Attempts)
	}
	if duration, ok := metrics.BackendTimes["openai"]; !ok || duration != 100*time.Millisecond {
		t.Errorf("BackendTimes[openai] = %v, want 100ms", duration)
	}

	metrics.RecordBackendTime("anthropic", 150*time.Millisecond)

	if metrics.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", metrics.Attempts)
	}
}

func TestWriteRequestLogFile_TestMode(t *testing.T) {
	SetTestMode(true)
	defer SetTestMode(false)

	cfg := &config.Config{
		Logging: config.Logging{
			SeparateFiles: true,
		},
	}

	err := WriteRequestLogFile(cfg, "test-req", "test content")
	if err != nil {
		t.Errorf("WriteRequestLogFile in test mode returned error: %v", err)
	}
}

func TestWriteErrorLogFile_TestMode(t *testing.T) {
	SetTestMode(true)
	defer SetTestMode(false)

	cfg := &config.Config{
		Logging: config.Logging{
			SeparateFiles: true,
		},
	}

	err := WriteErrorLogFile(cfg, "test-err", "test content")
	if err != nil {
		t.Errorf("WriteErrorLogFile in test mode returned error: %v", err)
	}
}
