package proxy

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProtocolDetection(t *testing.T) {

	t.Run("DetectProtocol_AnthropicByPath", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))
		req.Header.Set("x-api-key", "test-key")

		detector := NewRequestDetector()
		protocol := detector.DetectProtocol(req)

		if protocol != ProtocolAnthropic {
			t.Errorf("Expected ProtocolAnthropic, got %s", protocol)
		}
	})

	t.Run("DetectProtocol_AnthropicByVersion", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/unknown", strings.NewReader(`{}`))
		req.Header.Set("anthropic-version", "2023-06-01")

		detector := NewRequestDetector()
		protocol := detector.DetectProtocol(req)

		if protocol != ProtocolAnthropic {
			t.Errorf("Expected ProtocolAnthropic, got %s", protocol)
		}
	})

	t.Run("DetectProtocol_OpenAIByPath", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{}`))

		detector := NewRequestDetector()
		protocol := detector.DetectProtocol(req)

		if protocol != ProtocolOpenAI {
			t.Errorf("Expected ProtocolOpenAI, got %s", protocol)
		}
	})

	t.Run("DetectProtocol_OpenAIByBearer", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/unknown", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer sk-test")

		detector := NewRequestDetector()
		protocol := detector.DetectProtocol(req)

		if protocol != ProtocolOpenAI {
			t.Errorf("Expected ProtocolOpenAI, got %s", protocol)
		}
	})

	t.Run("DetectProtocol_DefaultToOpenAI", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/unknown", strings.NewReader(`{}`))

		detector := NewRequestDetector()
		protocol := detector.DetectProtocol(req)

		if protocol != ProtocolOpenAI {
			t.Errorf("Expected ProtocolOpenAI (default), got %s", protocol)
		}
	})
}
