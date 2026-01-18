package proxy

import (
	"strconv"
	"strings"

	"llm-proxy/config"
	"llm-proxy/logging"
)

type Detector struct {
	configMgr *config.Manager
}

func NewDetector(cfg *config.Manager) *Detector {
	return &Detector{configMgr: cfg}
}

func (d *Detector) ShouldFallback(statusCode int, body string) bool {
	cfg := d.configMgr.Get()

	errorCodes := cfg.Detection.ErrorCodes
	// Use default error codes if not configured
	if len(errorCodes) == 0 {
		errorCodes = []string{"4xx", "5xx"}
	}

	for _, pattern := range errorCodes {
		if d.matchStatusCode(statusCode, pattern) {
			logging.ProxySugar.Debugw("检测到错误状态码", "statusCode", statusCode, "pattern", pattern)
			return true
		}
	}

	for _, pattern := range cfg.Detection.ErrorPatterns {
		if strings.Contains(body, pattern) {
			logging.ProxySugar.Debugw("检测到错误模式", "statusCode", statusCode, "pattern", pattern)
			return true
		}
	}

	return false
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
