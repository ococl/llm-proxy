package proxy

import (
	"encoding/json"
	"strconv"
	"strings"

	"llm-proxy/config"
	"llm-proxy/logging"
)

type ErrorType int

const (
	ErrorTypeTransient ErrorType = iota
	ErrorTypePermanent
	ErrorTypeRateLimit
	ErrorTypeAuth
	ErrorTypeQuota
	ErrorTypeUnknown
)

type ErrorClassification struct {
	Type       ErrorType
	Retryable  bool
	StatusCode int
	Message    string
}

type Detector struct {
	configMgr *config.Manager
}

func NewDetector(cfg *config.Manager) *Detector {
	return &Detector{configMgr: cfg}
}

func (d *Detector) ClassifyError(statusCode int, body string) *ErrorClassification {
	classification := &ErrorClassification{
		StatusCode: statusCode,
		Type:       ErrorTypeUnknown,
		Retryable:  false,
		Message:    body,
	}

	switch {
	case statusCode == 429:
		classification.Type = ErrorTypeRateLimit
		classification.Retryable = true
	case statusCode == 401 || statusCode == 403:
		classification.Type = ErrorTypeAuth
		classification.Retryable = false
	case statusCode == 402:
		classification.Type = ErrorTypeQuota
		classification.Retryable = false
	case statusCode >= 500 && statusCode < 600:
		classification.Type = ErrorTypeTransient
		classification.Retryable = true
	case statusCode >= 400 && statusCode < 500:
		classification.Type = ErrorTypePermanent
		classification.Retryable = d.isPermanentErrorRetryable(body)
	default:
		classification.Retryable = false
	}

	var errorBody map[string]interface{}
	if err := json.Unmarshal([]byte(body), &errorBody); err == nil {
		if errObj, ok := errorBody["error"].(map[string]interface{}); ok {
			if errorType, ok := errObj["type"].(string); ok {
				classification = d.refineClassification(classification, errorType, body)
			}
		}
	}

	return classification
}

func (d *Detector) refineClassification(classification *ErrorClassification, errorType string, body string) *ErrorClassification {
	switch {
	case strings.Contains(errorType, "rate_limit"):
		classification.Type = ErrorTypeRateLimit
		classification.Retryable = true
	case strings.Contains(errorType, "insufficient_quota"):
		classification.Type = ErrorTypeQuota
		classification.Retryable = false
	case strings.Contains(errorType, "overloaded") || strings.Contains(errorType, "server_error"):
		classification.Type = ErrorTypeTransient
		classification.Retryable = true
	case strings.Contains(errorType, "invalid_request") || strings.Contains(errorType, "invalid_api_key"):
		classification.Type = ErrorTypePermanent
		classification.Retryable = false
	}

	for _, pattern := range []string{
		"insufficient_quota",
		"rate_limit",
		"overloaded",
		"unavailable",
		"timeout",
		"connection_error",
	} {
		if strings.Contains(strings.ToLower(body), pattern) {
			if pattern == "insufficient_quota" {
				classification.Type = ErrorTypeQuota
				classification.Retryable = false
			} else if pattern == "rate_limit" {
				classification.Type = ErrorTypeRateLimit
				classification.Retryable = true
			} else {
				classification.Type = ErrorTypeTransient
				classification.Retryable = true
			}
			break
		}
	}

	return classification
}

func (d *Detector) isPermanentErrorRetryable(body string) bool {
	retryablePatterns := []string{
		"context_length_exceeded",
		"model_not_found",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(body), pattern) {
			return true
		}
	}

	return false
}

func (d *Detector) ShouldFallback(statusCode int, body string) bool {
	cfg := d.configMgr.Get()

	errorCodes := cfg.Detection.ErrorCodes
	if len(errorCodes) == 0 {
		errorCodes = []string{"4xx", "5xx"}
	}

	logging.FileOnlySugar.Debugw("开始错误检测", "statusCode", statusCode, "errorCodes", errorCodes, "errorPatterns", cfg.Detection.ErrorPatterns, "bodyLength", len(body))

	classification := d.ClassifyError(statusCode, body)
	logging.FileOnlySugar.Debugw("错误分类",
		"statusCode", statusCode,
		"errorType", classification.Type,
		"retryable", classification.Retryable)

	for _, pattern := range errorCodes {
		if d.matchStatusCode(statusCode, pattern) {
			logging.ProxySugar.Debugw("检测到错误状态码,触发回退", "statusCode", statusCode, "pattern", pattern)
			return true
		}
	}

	for _, pattern := range cfg.Detection.ErrorPatterns {
		if strings.Contains(body, pattern) {
			logging.ProxySugar.Debugw("检测到错误模式,触发回退", "statusCode", statusCode, "pattern", pattern, "bodySnippet", truncateString(body, 200))
			return true
		}
	}

	logging.FileOnlySugar.Debugw("未匹配到任何错误规则,不触发回退", "statusCode", statusCode)
	return false
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (d *Detector) matchStatusCode(code int, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if strings.HasSuffix(pattern, "xx") {
		prefix := strings.TrimSuffix(pattern, "xx")
		codePrefix := strconv.Itoa(code / 100)
		return codePrefix == prefix
	}
	exact, err := strconv.Atoi(pattern)
	if err != nil {
		return false
	}
	return code == exact
}
