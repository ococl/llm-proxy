package main

import (
	"testing"
)

type mockConfigForDetector struct {
	detection Detection
}

func (m *mockConfigForDetector) getDetection() Detection {
	return m.detection
}

func newDetectorWithConfig(errorCodes []string, errorPatterns []string) *Detector {
	cfg := &Config{
		Detection: Detection{
			ErrorCodes:    errorCodes,
			ErrorPatterns: errorPatterns,
		},
	}
	cm := &ConfigManager{config: cfg}
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
		{502, false},
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
		{404, true},
		{499, true},
		{500, true},
		{502, true},
		{599, true},
		{200, false},
		{201, false},
		{301, false},
		{302, false},
	}

	for _, tt := range tests {
		got := d.ShouldFallback(tt.code, "")
		if got != tt.expected {
			t.Errorf("ShouldFallback(%d) with 4xx/5xx = %v, want %v", tt.code, got, tt.expected)
		}
	}
}

func TestDetector_MatchStatusCode_Mixed(t *testing.T) {
	d := newDetectorWithConfig([]string{"401", "5xx"}, nil)

	tests := []struct {
		code     int
		expected bool
	}{
		{401, true},
		{500, true},
		{502, true},
		{400, false},
		{403, false},
		{200, false},
	}

	for _, tt := range tests {
		got := d.ShouldFallback(tt.code, "")
		if got != tt.expected {
			t.Errorf("ShouldFallback(%d) with mixed = %v, want %v", tt.code, got, tt.expected)
		}
	}
}

func TestDetector_ErrorPatterns(t *testing.T) {
	d := newDetectorWithConfig(nil, []string{"insufficient_quota", "rate_limit", "exceeded"})

	tests := []struct {
		body     string
		expected bool
	}{
		{`{"error": "insufficient_quota"}`, true},
		{`{"error": "rate_limit exceeded"}`, true},
		{`quota exceeded for today`, true},
		{`{"status": "ok"}`, false},
		{``, false},
	}

	for _, tt := range tests {
		got := d.ShouldFallback(200, tt.body)
		if got != tt.expected {
			t.Errorf("ShouldFallback(200, %q) = %v, want %v", tt.body, got, tt.expected)
		}
	}
}

func TestDetector_Combined(t *testing.T) {
	d := newDetectorWithConfig([]string{"429"}, []string{"quota"})

	tests := []struct {
		code     int
		body     string
		expected bool
	}{
		{429, "", true},
		{200, "quota exceeded", true},
		{429, "quota exceeded", true},
		{200, "success", false},
	}

	for _, tt := range tests {
		got := d.ShouldFallback(tt.code, tt.body)
		if got != tt.expected {
			t.Errorf("ShouldFallback(%d, %q) = %v, want %v", tt.code, tt.body, got, tt.expected)
		}
	}
}

func TestDetector_InvalidPattern(t *testing.T) {
	d := newDetectorWithConfig([]string{"abc", "xxx", ""}, nil)

	if d.ShouldFallback(500, "") {
		t.Error("Invalid patterns should not match")
	}
}
