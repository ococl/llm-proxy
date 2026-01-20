package errors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONError(t *testing.T) {
	tests := []struct {
		name     string
		err      APIError
		status   int
		traceID  string
		wantCode int
	}{
		{
			name:     "bad request with trace ID",
			err:      ErrBadRequest,
			status:   http.StatusBadRequest,
			traceID:  "trace-123",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "unauthorized without trace ID",
			err:      ErrUnauthorized,
			status:   http.StatusUnauthorized,
			traceID:  "",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "rate limited",
			err:      ErrRateLimited,
			status:   http.StatusTooManyRequests,
			traceID:  "trace-456",
			wantCode: http.StatusTooManyRequests,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteJSONError(rec, tt.err, tt.status, tt.traceID)

			if rec.Code != tt.wantCode {
				t.Errorf("Expected status %d, got %d", tt.wantCode, rec.Code)
			}

			var resp APIError
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if resp.Code != tt.err.Code {
				t.Errorf("Expected code %s, got %s", tt.err.Code, resp.Code)
			}

			if tt.traceID != "" && resp.TraceID != tt.traceID {
				t.Errorf("Expected trace ID %s, got %s", tt.traceID, resp.TraceID)
			}
		})
	}
}

func TestWriteJSONErrorWithMsg(t *testing.T) {
	rec := httptest.NewRecorder()
	customMsg := "Custom error message"
	traceID := "trace-789"

	WriteJSONErrorWithMsg(rec, ErrNoBackend, http.StatusBadGateway, traceID, customMsg)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("Expected status 502, got %d", rec.Code)
	}

	var resp APIError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Message != customMsg {
		t.Errorf("Expected message %s, got %s", customMsg, resp.Message)
	}

	if resp.TraceID != traceID {
		t.Errorf("Expected trace ID %s, got %s", traceID, resp.TraceID)
	}
}

func TestAPIErrorConstants(t *testing.T) {
	tests := []struct {
		name     string
		err      APIError
		wantCode string
	}{
		{"ErrBadRequest", ErrBadRequest, "BAD_REQUEST"},
		{"ErrUnauthorized", ErrUnauthorized, "UNAUTHORIZED"},
		{"ErrMissingModel", ErrMissingModel, "MISSING_MODEL"},
		{"ErrInvalidJSON", ErrInvalidJSON, "INVALID_JSON"},
		{"ErrUnknownModel", ErrUnknownModel, "UNKNOWN_MODEL"},
		{"ErrNoBackend", ErrNoBackend, "NO_BACKEND"},
		{"ErrRateLimited", ErrRateLimited, "RATE_LIMITED"},
		{"ErrConcurrencyLimit", ErrConcurrencyLimit, "CONCURRENCY_LIMIT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("Expected code %s, got %s", tt.wantCode, tt.err.Code)
			}
			if tt.err.Message == "" {
				t.Error("Error message should not be empty")
			}
		})
	}
}
