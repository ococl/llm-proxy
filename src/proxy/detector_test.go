package proxy

import (
	"testing"

	"llm-proxy/config"
)

func newDetectorWithConfig(errorCodes []string, errorPatterns []string) *Detector {
	cfg := &config.Config{
		Detection: config.Detection{
			ErrorCodes:    errorCodes,
			ErrorPatterns: errorPatterns,
		},
	}
	cm := newTestManager(cfg)
	return NewDetector(cm)
}

func TestDetector_MatchStatusCode_Exact(t *testing.T) {
	d := newDetectorWithConfig([]string{"401", "403", "500"}, nil)

	tests := []struct {
		code     int
		expected bool
	}{
		{401, true},
		{403, true},
		{500, true},
		{200, false},
		{404, false},
	}

	for _, tt := range tests {
		got := d.ShouldFallback(tt.code, "")
		if got != tt.expected {
			t.Errorf("ShouldFallback(%d) = %v, want %v", tt.code, got, tt.expected)
		}
	}
}

func TestDetector_MatchStatusCode_Wildcard(t *testing.T) {
	d := newDetectorWithConfig([]string{"4xx", "5xx"}, nil)

	tests := []struct {
		code     int
		expected bool
	}{
		{400, true},
		{401, true},
		{500, true},
		{200, false},
		{301, false},
	}

	for _, tt := range tests {
		got := d.ShouldFallback(tt.code, "")
		if got != tt.expected {
			t.Errorf("ShouldFallback(%d) with 4xx/5xx = %v, want %v", tt.code, got, tt.expected)
		}
	}
}

func TestDetector_ErrorPatterns(t *testing.T) {
	d := newDetectorWithConfig(nil, []string{"insufficient_quota", "rate_limit"})

	tests := []struct {
		body     string
		expected bool
	}{
		{`{"error": "insufficient_quota"}`, true},
		{`{"error": "rate_limit exceeded"}`, true},
		{`{"status": "ok"}`, false},
	}

	for _, tt := range tests {
		got := d.ShouldFallback(200, tt.body)
		if got != tt.expected {
			t.Errorf("ShouldFallback(200, %q) = %v, want %v", tt.body, got, tt.expected)
		}
	}
}
