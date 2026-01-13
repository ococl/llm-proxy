package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	generalLogger *os.File
	logMu         sync.Mutex
	logLevel      = "debug"
	testMode      = false
)

func SetTestMode(enabled bool) {
	testMode = enabled
}

var levelPriority = map[string]int{
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
}

func InitLogger(cfg *Config) error {
	logLevel = cfg.Logging.Level
	if err := os.MkdirAll(cfg.Logging.RequestDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Logging.ErrorDir, 0755); err != nil {
		return err
	}
	dir := filepath.Dir(cfg.Logging.GeneralFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(cfg.Logging.GeneralFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	generalLogger = f
	return nil
}

func LogGeneral(level, format string, args ...interface{}) {
	if testMode {
		return
	}
	if levelPriority[strings.ToLower(level)] < levelPriority[logLevel] {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] [%s] %s\n", time.Now().Format(time.RFC3339), level, msg)
	fmt.Print(line)
	if generalLogger != nil {
		generalLogger.WriteString(line)
	}
}

func WriteRequestLog(cfg *Config, reqID string, content string) error {
	if testMode {
		return nil
	}
	filename := filepath.Join(cfg.Logging.RequestDir, reqID+".log")
	return os.WriteFile(filename, []byte(content), 0644)
}

func WriteErrorLog(cfg *Config, reqID string, content string) error {
	if testMode {
		return nil
	}
	filename := filepath.Join(cfg.Logging.ErrorDir, reqID+".log")
	return os.WriteFile(filename, []byte(content), 0644)
}
