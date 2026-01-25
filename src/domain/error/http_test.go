package domainerror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestToAPIResponse(t *testing.T) {
	t.Run("Converts error to API response", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "参数错误").
			WithReqID("req-123").
			WithBackend("openai")

		resp := ToAPIResponse(err)

		if resp.Error.Code != string(CodeBadRequest) {
			t.Errorf("Expected code '%s', got '%s'", CodeBadRequest, resp.Error.Code)
		}
		if resp.Error.Message != "参数错误" {
			t.Errorf("Expected message '参数错误', got '%s'", resp.Error.Message)
		}
		if resp.Error.Type != string(ErrorTypeClient) {
			t.Errorf("Expected type '%s', got '%s'", ErrorTypeClient, resp.Error.Type)
		}
		if resp.Error.ReqID != "req-123" {
			t.Errorf("Expected req_id 'req-123', got '%s'", resp.Error.ReqID)
		}
		if resp.Error.Backend != "openai" {
			t.Errorf("Expected backend 'openai', got '%s'", resp.Error.Backend)
		}
	})
}

func TestWriteJSONError(t *testing.T) {
	t.Run("Writes JSON error with correct status", func(t *testing.T) {
		rec := httptest.NewRecorder()

		err := New(ErrorTypeClient, CodeBadRequest, "测试错误").WithReqID("req-123")
		WriteJSONError(rec, err)

		if rec.Code != 400 {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}

		var resp APIErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if resp.Error.Code != string(CodeBadRequest) {
			t.Errorf("Expected code '%s', got '%s'", CodeBadRequest, resp.Error.Code)
		}
	})

	t.Run("Sets content type header", func(t *testing.T) {
		rec := httptest.NewRecorder()

		err := New(ErrorTypeClient, CodeBadRequest, "测试错误")
		WriteJSONError(rec, err)

		if rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", rec.Header().Get("Content-Type"))
		}
	})
}

func TestWriteError(t *testing.T) {
	t.Run("Writes LLMProxyError correctly", func(t *testing.T) {
		rec := httptest.NewRecorder()

		proxyErr := New(ErrorTypeClient, CodeBadRequest, "参数错误")
		WriteError(rec, proxyErr)

		if rec.Code != 400 {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("Creates internal error for non-LLMProxyError", func(t *testing.T) {
		rec := httptest.NewRecorder()

		regularErr := &testError{msg: "some error"}
		WriteError(rec, regularErr)

		if rec.Code != 500 {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}
	})
}

// testError 用于测试的非 LLMProxyError 类型
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestWriteAPIError(t *testing.T) {
	t.Run("Writes simple API error", func(t *testing.T) {
		rec := httptest.NewRecorder()

		WriteAPIError(rec, "CUSTOM_ERROR", "自定义错误", http.StatusBadRequest)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}

		var resp APIErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if resp.Error.Code != "CUSTOM_ERROR" {
			t.Errorf("Expected code 'CUSTOM_ERROR', got '%s'", resp.Error.Code)
		}
		if resp.Error.Message != "自定义错误" {
			t.Errorf("Expected message '自定义错误', got '%s'", resp.Error.Message)
		}
	})
}

func TestWriteAPIErrorWithReqID(t *testing.T) {
	t.Run("Writes API error with request ID", func(t *testing.T) {
		rec := httptest.NewRecorder()

		WriteAPIErrorWithReqID(rec, "ERROR_CODE", "错误消息", "req-456", http.StatusBadRequest)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}

		var resp APIErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if resp.Error.ReqID != "req-456" {
			t.Errorf("Expected req_id 'req-456', got '%s'", resp.Error.ReqID)
		}
	})
}

func TestWriteBadRequest(t *testing.T) {
	t.Run("Writes bad request error", func(t *testing.T) {
		rec := httptest.NewRecorder()

		WriteBadRequest(rec, "参数无效")

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

func TestWriteUnauthorized(t *testing.T) {
	t.Run("Writes unauthorized error", func(t *testing.T) {
		rec := httptest.NewRecorder()

		WriteUnauthorized(rec, "API Key 无效")

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})
}

func TestWriteRateLimited(t *testing.T) {
	t.Run("Writes rate limited error", func(t *testing.T) {
		rec := httptest.NewRecorder()

		WriteRateLimited(rec)

		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("Expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
		}
	})
}

func TestWriteConcurrencyLimit(t *testing.T) {
	t.Run("Writes concurrency limit error", func(t *testing.T) {
		rec := httptest.NewRecorder()

		WriteConcurrencyLimit(rec)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
		}
	})
}

func TestWriteBackendError(t *testing.T) {
	t.Run("Writes backend error with message", func(t *testing.T) {
		rec := httptest.NewRecorder()

		WriteBackendError(rec, "openai", "req-789")

		if rec.Code != http.StatusBadGateway {
			t.Errorf("Expected status %d, got %d", http.StatusBadGateway, rec.Code)
		}

		var resp APIErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		// Backend name is included in message
		if resp.Error.Message != "后端 openai 请求失败" {
			t.Errorf("Expected message '后端 openai 请求失败', got '%s'", resp.Error.Message)
		}
	})
}

func TestWriteNoBackend(t *testing.T) {
	t.Run("Writes no backend error", func(t *testing.T) {
		rec := httptest.NewRecorder()

		WriteNoBackend(rec, "req-999")

		if rec.Code != http.StatusBadGateway {
			t.Errorf("Expected status %d, got %d", http.StatusBadGateway, rec.Code)
		}

		var resp APIErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Errorf("Failed to unmarshal response: %v", err)
		}

		if resp.Error.Code != "NO_BACKEND" {
			t.Errorf("Expected code 'NO_BACKEND', got '%s'", resp.Error.Code)
		}
		if resp.Error.ReqID != "req-999" {
			t.Errorf("Expected req_id 'req-999', got '%s'", resp.Error.ReqID)
		}
	})
}
