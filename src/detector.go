package main

import (
	"strings"
)

type Detector struct {
	configMgr *ConfigManager
}

func NewDetector(cfg *ConfigManager) *Detector {
	return &Detector{configMgr: cfg}
}

func (d *Detector) ShouldFallback(statusCode int, body string) bool {
	cfg := d.configMgr.Get()

	for _, code := range cfg.Detection.ErrorCodes {
		if statusCode == code {
			return true
		}
	}

	for _, pattern := range cfg.Detection.ErrorPatterns {
		if strings.Contains(body, pattern) {
			return true
		}
	}

	return false
}
