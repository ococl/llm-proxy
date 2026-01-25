package domainerror

import (
	"errors"
	"testing"
)

func TestLLMProxyError(t *testing.T) {
	t.Run("Error returns message without cause", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "无效的请求")
		expected := "[client:BAD_REQUEST] 无效的请求"
		if err.Error() != expected {
			t.Errorf("Expected '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("Error returns message with cause", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := Wrap(originalErr, ErrorTypeBackend, CodeBackendError, "后端请求失败")
		result := err.Error()
		if result == "" {
			t.Error("Expected non-empty error message")
		}
	})

	t.Run("Unwrap returns cause", func(t *testing.T) {
		originalErr := errors.New("original")
		err := Wrap(originalErr, ErrorTypeBackend, CodeBackendError, "test")
		if err.Unwrap() != originalErr {
			t.Error("Expected Unwrap to return original cause")
		}
	})

	t.Run("Is matches by code", func(t *testing.T) {
		err1 := New(ErrorTypeClient, CodeBadRequest, "error 1")
		err2 := New(ErrorTypeClient, CodeBadRequest, "error 2")
		target := &LLMProxyError{Code: CodeBadRequest}

		if !errors.Is(err1, target) {
			t.Error("Expected errors to match by code")
		}
		if !errors.Is(err2, target) {
			t.Error("Expected errors to match by code")
		}
	})

	t.Run("Is returns false for non-LLMProxyError", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "test")
		regularErr := errors.New("regular error")
		if errors.Is(err, regularErr) {
			t.Error("Expected Is to return false for non-LLMProxyError")
		}
	})
}

func TestLLMProxyErrorWithReqID(t *testing.T) {
	t.Run("WithReqID adds request ID", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "test").WithReqID("req-123")
		if err.ReqID != "req-123" {
			t.Errorf("Expected 'req-123', got '%s'", err.ReqID)
		}
	})
}

func TestLLMProxyErrorWithBackend(t *testing.T) {
	t.Run("WithBackend adds backend name", func(t *testing.T) {
		err := New(ErrorTypeBackend, CodeBackendError, "test").WithBackend("openai")
		if err.BackendName != "openai" {
			t.Errorf("Expected 'openai', got '%s'", err.BackendName)
		}
	})
}

func TestLLMProxyErrorWithCause(t *testing.T) {
	t.Run("WithCause adds cause", func(t *testing.T) {
		cause := errors.New("original cause")
		err := New(ErrorTypeBackend, CodeBackendError, "test").WithCause(cause)
		if err.Cause != cause {
			t.Error("Expected cause to be set")
		}
	})
}

func TestGetHTTPStatus(t *testing.T) {
	t.Run("Client error returns 400", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "test")
		if err.GetHTTPStatus() != 400 {
			t.Errorf("Expected 400, got %d", err.GetHTTPStatus())
		}
	})

	t.Run("Validation error returns 400", func(t *testing.T) {
		err := New(ErrorTypeValidation, CodeInvalidRequest, "test")
		if err.GetHTTPStatus() != 400 {
			t.Errorf("Expected 400, got %d", err.GetHTTPStatus())
		}
	})

	t.Run("Rate limit error returns 429", func(t *testing.T) {
		err := New(ErrorTypeRateLimit, CodeRateLimited, "test")
		if err.GetHTTPStatus() != 429 {
			t.Errorf("Expected 429, got %d", err.GetHTTPStatus())
		}
	})

	t.Run("Concurrency limit returns 503", func(t *testing.T) {
		err := New(ErrorTypeConcurrency, CodeConcurrencyLimit, "test")
		if err.GetHTTPStatus() != 503 {
			t.Errorf("Expected 503, got %d", err.GetHTTPStatus())
		}
	})

	t.Run("Backend error returns 502", func(t *testing.T) {
		err := New(ErrorTypeBackend, CodeBackendError, "test")
		if err.GetHTTPStatus() != 502 {
			t.Errorf("Expected 502, got %d", err.GetHTTPStatus())
		}
	})

	t.Run("Internal error returns 500", func(t *testing.T) {
		err := New(ErrorTypeInternal, CodeInternal, "test")
		if err.GetHTTPStatus() != 500 {
			t.Errorf("Expected 500, got %d", err.GetHTTPStatus())
		}
	})

	t.Run("Protocol error returns 500", func(t *testing.T) {
		err := New(ErrorTypeProtocol, CodeProtocolConvert, "test")
		if err.GetHTTPStatus() != 500 {
			t.Errorf("Expected 500, got %d", err.GetHTTPStatus())
		}
	})

	t.Run("Config error returns 500", func(t *testing.T) {
		err := New(ErrorTypeConfig, CodeConfigLoad, "test")
		if err.GetHTTPStatus() != 500 {
			t.Errorf("Expected 500, got %d", err.GetHTTPStatus())
		}
	})

	t.Run("Custom HTTP status overrides default", func(t *testing.T) {
		err := NewWithStatus(ErrorTypeClient, CodeBadRequest, "test", 418)
		if err.GetHTTPStatus() != 418 {
			t.Errorf("Expected 418, got %d", err.GetHTTPStatus())
		}
	})

	t.Run("Unknown type returns 500", func(t *testing.T) {
		err := New("unknown", CodeUnknown, "test")
		if err.GetHTTPStatus() != 500 {
			t.Errorf("Expected 500, got %d", err.GetHTTPStatus())
		}
	})
}

func TestNew(t *testing.T) {
	t.Run("New creates error with correct fields", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "测试消息")
		if err.Type != ErrorTypeClient {
			t.Errorf("Expected ErrorTypeClient, got '%s'", err.Type)
		}
		if err.Code != CodeBadRequest {
			t.Errorf("Expected CodeBadRequest, got '%s'", err.Code)
		}
		if err.Message != "测试消息" {
			t.Errorf("Expected '测试消息', got '%s'", err.Message)
		}
	})
}

func TestNewWithStatus(t *testing.T) {
	t.Run("NewWithStatus creates error with custom status", func(t *testing.T) {
		err := NewWithStatus(ErrorTypeClient, CodeBadRequest, "test", 422)
		if err.HTTPStatus != 422 {
			t.Errorf("Expected 422, got %d", err.HTTPStatus)
		}
	})
}

func TestWrap(t *testing.T) {
	t.Run("Wrap creates error with cause", func(t *testing.T) {
		original := errors.New("original error")
		err := Wrap(original, ErrorTypeBackend, CodeBackendError, "后端失败")
		if err.Cause != original {
			t.Error("Expected cause to be set")
		}
		if err.Type != ErrorTypeBackend {
			t.Errorf("Expected ErrorTypeBackend, got '%s'", err.Type)
		}
	})
}

func TestIsType(t *testing.T) {
	t.Run("Returns true for matching type", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "test")
		if !IsType(err, ErrorTypeClient) {
			t.Error("Expected IsType to return true")
		}
	})

	t.Run("Returns false for non-matching type", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "test")
		if IsType(err, ErrorTypeBackend) {
			t.Error("Expected IsType to return false")
		}
	})

	t.Run("Returns false for non-LLMProxyError", func(t *testing.T) {
		regularErr := errors.New("error")
		if IsType(regularErr, ErrorTypeClient) {
			t.Error("Expected IsType to return false for regular error")
		}
	})
}

func TestIsCode(t *testing.T) {
	t.Run("Returns true for matching code", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "test")
		if !IsCode(err, CodeBadRequest) {
			t.Error("Expected IsCode to return true")
		}
	})

	t.Run("Returns false for non-matching code", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "test")
		if IsCode(err, CodeUnauthorized) {
			t.Error("Expected IsCode to return false")
		}
	})

	t.Run("Returns false for non-LLMProxyError", func(t *testing.T) {
		regularErr := errors.New("error")
		if IsCode(regularErr, CodeBadRequest) {
			t.Error("Expected IsCode to return false for regular error")
		}
	})
}

func TestGetReqID(t *testing.T) {
	t.Run("Returns request ID from error", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "test").WithReqID("req-abc")
		if GetReqID(err) != "req-abc" {
			t.Errorf("Expected 'req-abc', got '%s'", GetReqID(err))
		}
	})

	t.Run("Returns empty string for error without req ID", func(t *testing.T) {
		err := New(ErrorTypeClient, CodeBadRequest, "test")
		if GetReqID(err) != "" {
			t.Errorf("Expected empty string, got '%s'", GetReqID(err))
		}
	})

	t.Run("Returns empty for non-LLMProxyError", func(t *testing.T) {
		regularErr := errors.New("error")
		if GetReqID(regularErr) != "" {
			t.Error("Expected empty string for regular error")
		}
	})
}

func TestGetBackendName(t *testing.T) {
	t.Run("Returns backend name from error", func(t *testing.T) {
		err := New(ErrorTypeBackend, CodeBackendError, "test").WithBackend("openai")
		if GetBackendName(err) != "openai" {
			t.Errorf("Expected 'openai', got '%s'", GetBackendName(err))
		}
	})

	t.Run("Returns empty for error without backend name", func(t *testing.T) {
		err := New(ErrorTypeBackend, CodeBackendError, "test")
		if GetBackendName(err) != "" {
			t.Errorf("Expected empty string, got '%s'", GetBackendName(err))
		}
	})

	t.Run("Returns empty for non-LLMProxyError", func(t *testing.T) {
		regularErr := errors.New("error")
		if GetBackendName(regularErr) != "" {
			t.Error("Expected empty string for regular error")
		}
	})
}

func TestConvenienceConstructors(t *testing.T) {
	t.Run("ErrBadRequest", func(t *testing.T) {
		if ErrBadRequest.Code != CodeBadRequest {
			t.Error("Expected ErrBadRequest to have CodeBadRequest")
		}
	})

	t.Run("ErrUnauthorized", func(t *testing.T) {
		if ErrUnauthorized.Code != CodeUnauthorized {
			t.Error("Expected ErrUnauthorized to have CodeUnauthorized")
		}
	})

	t.Run("ErrMissingModel", func(t *testing.T) {
		if ErrMissingModel.Code != CodeMissingModel {
			t.Error("Expected ErrMissingModel to have CodeMissingModel")
		}
	})

	t.Run("ErrInvalidJSON", func(t *testing.T) {
		if ErrInvalidJSON.Code != CodeInvalidJSON {
			t.Error("Expected ErrInvalidJSON to have CodeInvalidJSON")
		}
	})

	t.Run("ErrUnknownModel", func(t *testing.T) {
		if ErrUnknownModel.Code != CodeUnknownModel {
			t.Error("Expected ErrUnknownModel to have CodeUnknownModel")
		}
	})

	t.Run("ErrNoBackend", func(t *testing.T) {
		if ErrNoBackend.Code != CodeNoBackend {
			t.Error("Expected ErrNoBackend to have CodeNoBackend")
		}
	})

	t.Run("ErrRateLimited", func(t *testing.T) {
		if ErrRateLimited.Code != CodeRateLimited {
			t.Error("Expected ErrRateLimited to have CodeRateLimited")
		}
	})

	t.Run("ErrConcurrencyLimit", func(t *testing.T) {
		if ErrConcurrencyLimit.Code != CodeConcurrencyLimit {
			t.Error("Expected ErrConcurrencyLimit to have CodeConcurrencyLimit")
		}
	})

	t.Run("ErrInvalidRequest", func(t *testing.T) {
		if ErrInvalidRequest.Code != CodeInvalidRequest {
			t.Error("Expected ErrInvalidRequest to have CodeInvalidRequest")
		}
	})
}

func TestNewBadRequest(t *testing.T) {
	err := NewBadRequest("参数错误")
	if err.Type != ErrorTypeClient {
		t.Errorf("Expected ErrorTypeClient, got '%s'", err.Type)
	}
	if err.Code != CodeBadRequest {
		t.Errorf("Expected CodeBadRequest, got '%s'", err.Code)
	}
	if err.Message != "参数错误" {
		t.Errorf("Expected '参数错误', got '%s'", err.Message)
	}
}

func TestNewInvalidRequest(t *testing.T) {
	err := NewInvalidRequest("无效的模型: %s", "unknown")
	if err.Type != ErrorTypeValidation {
		t.Errorf("Expected ErrorTypeValidation, got '%s'", err.Type)
	}
	if err.Message != "无效的模型: unknown" {
		t.Errorf("Expected '无效的模型: unknown', got '%s'", err.Message)
	}
}

func TestNewUnauthorized(t *testing.T) {
	err := NewUnauthorized("API Key 无效")
	if err.Type != ErrorTypeClient {
		t.Errorf("Expected ErrorTypeClient, got '%s'", err.Type)
	}
	if err.HTTPStatus != 401 {
		t.Errorf("Expected 401, got %d", err.HTTPStatus)
	}
}

func TestNewMissingModel(t *testing.T) {
	err := NewMissingModel()
	if err.Code != CodeMissingModel {
		t.Errorf("Expected CodeMissingModel, got '%s'", err.Code)
	}
}

func TestNewInvalidJSON(t *testing.T) {
	original := errors.New("invalid json syntax")
	err := NewInvalidJSON(original)
	if err.Code != CodeInvalidJSON {
		t.Errorf("Expected CodeInvalidJSON, got '%s'", err.Code)
	}
	if err.Cause != original {
		t.Error("Expected cause to be set")
	}
}

func TestNewUnknownModel(t *testing.T) {
	err := NewUnknownModel("gpt-5")
	if err.Code != CodeUnknownModel {
		t.Errorf("Expected CodeUnknownModel, got '%s'", err.Code)
	}
	if err.Message != "未知的模型别名: gpt-5" {
		t.Errorf("Unexpected message: %s", err.Message)
	}
}

func TestNewNoBackend(t *testing.T) {
	err := NewNoBackend()
	if err.Code != CodeNoBackend {
		t.Errorf("Expected CodeNoBackend, got '%s'", err.Code)
	}
	if err.HTTPStatus != 502 {
		t.Errorf("Expected 502, got %d", err.HTTPStatus)
	}
}

func TestNewRateLimited(t *testing.T) {
	err := NewRateLimited("超出速率限制")
	if err.Code != CodeRateLimited {
		t.Errorf("Expected CodeRateLimited, got '%s'", err.Code)
	}
	if err.HTTPStatus != 429 {
		t.Errorf("Expected 429, got %d", err.HTTPStatus)
	}
}

func TestNewConcurrencyLimit(t *testing.T) {
	err := NewConcurrencyLimit("超出并发限制")
	if err.Code != CodeConcurrencyLimit {
		t.Errorf("Expected CodeConcurrencyLimit, got '%s'", err.Code)
	}
	if err.HTTPStatus != 503 {
		t.Errorf("Expected 503, got %d", err.HTTPStatus)
	}
}

func TestNewBackendError(t *testing.T) {
	original := errors.New("connection timeout")
	err := NewBackendError("openai", original)
	if err.Code != CodeBackendError {
		t.Errorf("Expected CodeBackendError, got '%s'", err.Code)
	}
	if err.BackendName != "openai" {
		t.Errorf("Expected backend name 'openai', got '%s'", err.BackendName)
	}
	if err.Cause != original {
		t.Error("Expected cause to be set")
	}
}

func TestNewProtocolError(t *testing.T) {
	original := errors.New("parse error")
	err := NewProtocolError("协议转换失败", original)
	if err.Code != CodeProtocolConvert {
		t.Errorf("Expected CodeProtocolConvert, got '%s'", err.Code)
	}
	if err.Type != ErrorTypeProtocol {
		t.Errorf("Expected ErrorTypeProtocol, got '%s'", err.Type)
	}
}

func TestNewConfigError(t *testing.T) {
	original := errors.New("file not found")
	err := NewConfigError("配置文件加载失败", original)
	if err.Code != CodeConfigLoad {
		t.Errorf("Expected CodeConfigLoad, got '%s'", err.Code)
	}
	if err.Type != ErrorTypeConfig {
		t.Errorf("Expected ErrorTypeConfig, got '%s'", err.Type)
	}
}

func TestNewInternalError(t *testing.T) {
	original := errors.New("nil pointer")
	err := NewInternalError("内部错误", original)
	if err.Code != CodeInternal {
		t.Errorf("Expected CodeInternal, got '%s'", err.Code)
	}
	if err.Type != ErrorTypeInternal {
		t.Errorf("Expected ErrorTypeInternal, got '%s'", err.Type)
	}
}
