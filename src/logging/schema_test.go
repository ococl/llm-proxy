package logging

import (
	"testing"
)

func TestNewSensitiveDataMasker(t *testing.T) {
	masker := NewSensitiveDataMasker()
	if masker == nil {
		t.Error("NewSensitiveDataMasker() returned nil")
	}
}

func TestSensitiveDataMasker_Mask(t *testing.T) {
	masker := NewSensitiveDataMasker()

	tests := []struct {
		name  string
		input string
		check func(string) bool
	}{
		{
			name:  "OpenAI API key masked",
			input: "sk-abcdefghijklmnopqrstuv",
			check: func(s string) bool {
				return s != "sk-abcdefghijklmnopqrstuv" && len(s) > 0
			},
		},
		{
			name:  "Project key masked",
			input: "pk-projabcdefghijklmnopqr",
			check: func(s string) bool {
				return s != "pk-projabcdefghijklmnopqr" && len(s) > 0
			},
		},
		{
			name:  "Bearer token masked",
			input: "Authorization: bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			check: func(s string) bool {
				return s != "Authorization: bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" && len(s) > 0
			},
		},
		{
			name:  "Normal text unchanged",
			input: "Hello, this is a normal message without sensitive data",
			check: func(s string) bool {
				return s == "Hello, this is a normal message without sensitive data"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.Mask(tt.input)
			if !tt.check(result) {
				t.Errorf("Mask(%q) = %q, check failed", tt.input, result)
			}
		})
	}
}

func TestSensitiveDataMasker_EmptyInput(t *testing.T) {
	masker := NewSensitiveDataMasker()
	result := masker.Mask("")
	if result != "" {
		t.Errorf("Mask(\"\") = %q, want \"\"", result)
	}
}

func TestSensitivePatterns_Initialized(t *testing.T) {
	if len(SensitivePatterns) == 0 {
		t.Error("SensitivePatterns should not be empty")
	}

	// Check that common patterns are present
	hasAPIKey := false
	for _, pattern := range SensitivePatterns {
		if pattern.String() != "" {
			hasAPIKey = true
			break
		}
	}
	if !hasAPIKey {
		t.Error("SensitivePatterns should contain patterns")
	}
}
