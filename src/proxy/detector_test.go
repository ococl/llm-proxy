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
		{"status: ok", false},
	}

	for _, tt := range tests {
		got := d.ShouldFallback(200, tt.body)
		if got != tt.expected {
			t.Errorf("ShouldFallback(200, %q) = %v, want %v", tt.body, got, tt.expected)
		}
	}
}

func TestDetector_429And500_Focus(t *testing.T) {
	d := newDetectorWithConfig([]string{"4xx", "5xx"}, nil)

	tests := []struct {
		name       string
		statusCode int
		body       string
		expected   bool
	}{
		{"429 Too Many Requests", 429, "", true},
		{"500 Internal Server Error", 500, "", true},
		{"503 Service Unavailable", 503, "", true},
		{"502 Bad Gateway", 502, "", true},
		{"504 Gateway Timeout", 504, "", true},
		{"400 Bad Request", 400, "", true},
		{"200 OK", 200, "", false},
		{"201 Created", 201, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.ShouldFallback(tt.statusCode, tt.body)
			if got != tt.expected {
				t.Errorf("ShouldFallback(%d, %q) = %v, want %v",
					tt.statusCode, tt.body, got, tt.expected)
			}
		})
	}
}

func TestDetector_EmptyConfig(t *testing.T) {
	// After fix: when error_codes is not configured, defaults to ["4xx", "5xx"]
	// So 500 and 429 errors should now trigger fallback
	d := newDetectorWithConfig([]string{}, nil)

	tests := []struct {
		name       string
		statusCode int
		body       string
		expected   bool
	}{
		// With default config (from fix), 500 SHOULD trigger fallback
		{"500 with default config", 500, "", true},
		// With default config (from fix), 429 SHOULD trigger fallback
		{"429 with default config", 429, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.ShouldFallback(tt.statusCode, tt.body)
			if got != tt.expected {
				t.Errorf("ShouldFallback(%d, %q) with default config = %v, want %v",
					tt.statusCode, tt.body, got, tt.expected)
			}
		})
	}
}
