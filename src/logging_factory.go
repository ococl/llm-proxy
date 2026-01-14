package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// markdownConsoleEncoder 自定义的Markdown风格控制台编码器
type markdownConsoleEncoder struct {
	zapcore.Encoder
	colored bool
}

// reqIDPattern 匹配请求ID的正则表达式
var reqIDPattern = regexp.MustCompile(`\[req_[a-zA-Z0-9]+\]`)

// newMarkdownConsoleEncoder 创建Markdown风格的控制台编码器
func newMarkdownConsoleEncoder(cfg zapcore.EncoderConfig, colored bool) zapcore.Encoder {
	return &markdownConsoleEncoder{
		Encoder: zapcore.NewConsoleEncoder(cfg),
		colored: colored,
	}
}

// Clone 克隆编码器
func (enc *markdownConsoleEncoder) Clone() zapcore.Encoder {
	return &markdownConsoleEncoder{
		Encoder: enc.Encoder.Clone(),
		colored: enc.colored,
	}
}

func (enc *markdownConsoleEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	line := bytes.NewBuffer(nil)

	timeStr := entry.Time.Format("15:04:05")
	if enc.colored {
		line.WriteString("\033[90m")
		line.WriteString(timeStr)
		line.WriteString("\033[0m")
	} else {
		line.WriteString(timeStr)
	}
	line.WriteString("  ")

	levelStr := entry.Level.CapitalString()
	if enc.colored {
		switch entry.Level {
		case zapcore.DebugLevel:
			line.WriteString("\033[35m")
		case zapcore.InfoLevel:
			line.WriteString("\033[32m")
		case zapcore.WarnLevel:
			line.WriteString("\033[33m")
		case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
			line.WriteString("\033[31m")
		}
		line.WriteString(levelStr)
		line.WriteString("\033[0m")
	} else {
		line.WriteString(levelStr)
	}
	line.WriteString("  ")

	msg := entry.Message
	if enc.colored {
		msg = reqIDPattern.ReplaceAllStringFunc(msg, func(match string) string {
			return "\033[36m" + match + "\033[0m"
		})
	}
	line.WriteString(msg)

	if len(fields) > 0 {
		line.WriteString("  ")
		firstField := true
		for _, field := range fields {
			if field.Key == "logger" {
				continue
			}

			if !firstField {
				line.WriteString(" ")
			}
			firstField = false

			if enc.colored {
				line.WriteString("\033[90m")
				line.WriteString(field.Key)
				line.WriteString("\033[0m=")
			} else {
				line.WriteString(field.Key)
				line.WriteString("=")
			}

			switch field.Type {
			case zapcore.StringType:
				if enc.colored {
					line.WriteString("\033[33m")
				}
				line.WriteString(field.String)
				if enc.colored {
					line.WriteString("\033[0m")
				}
			case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type:
				line.WriteString(fmt.Sprintf("%d", field.Integer))
			case zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type:
				line.WriteString(fmt.Sprintf("%d", field.Integer))
			case zapcore.Float64Type, zapcore.Float32Type:
				line.WriteString(fmt.Sprintf("%f", field.Integer))
			case zapcore.BoolType:
				if field.Integer == 1 {
					line.WriteString("true")
				} else {
					line.WriteString("false")
				}
			default:
				line.WriteString(field.String)
			}
		}
	}

	line.WriteString("\n")

	buf := buffer.NewPool().Get()
	buf.Write(line.Bytes())
	return buf, nil
}

// InitLoggers 初始化所有Logger实例
func InitLoggers(cfg *Config) error {
	baseDir := cfg.Logging.BaseDir
	if baseDir == "" {
		baseDir = "./logs"
	}

	// 确保日志根目录存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("创建日志根目录失败 %s: %w", baseDir, err)
	}

	// 创建各子目录
	dirs := []string{
		filepath.Join(baseDir, "system"),
		filepath.Join(baseDir, "network"),
		filepath.Join(baseDir, "proxy"),
		filepath.Join(baseDir, "llm_debug"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建日志子目录失败 %s: %w", dir, err)
		}
	}

	var err error

	// 初始化各Logger
	GeneralLogger, GeneralSugar, err = createLogger(cfg, "general", filepath.Join(baseDir, "general.log"))
	if err != nil {
		return fmt.Errorf("初始化GeneralLogger失败: %w", err)
	}

	SystemLogger, SystemSugar, err = createLogger(cfg, "system", filepath.Join(baseDir, "system", "system.log"))
	if err != nil {
		return fmt.Errorf("初始化SystemLogger失败: %w", err)
	}

	NetworkLogger, NetworkSugar, err = createLogger(cfg, "network", filepath.Join(baseDir, "network", "network.log"))
	if err != nil {
		return fmt.Errorf("初始化NetworkLogger失败: %w", err)
	}

	ProxyLogger, ProxySugar, err = createLogger(cfg, "proxy", filepath.Join(baseDir, "proxy", "proxy.log"))
	if err != nil {
		return fmt.Errorf("初始化ProxyLogger失败: %w", err)
	}

	DebugLogger, DebugSugar, err = createLogger(cfg, "debug", filepath.Join(baseDir, "llm_debug", "debug.log"))
	if err != nil {
		return fmt.Errorf("初始化DebugLogger失败: %w", err)
	}

	return nil
}

// createLogger 创建单个Logger实例
func createLogger(cfg *Config, name, filePath string) (*zap.Logger, *zap.SugaredLogger, error) {
	// 创建日志轮转配置
	fw := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    cfg.Logging.GetMaxFileSizeMB(),
		MaxAge:     cfg.Logging.GetMaxAgeDays(),
		MaxBackups: cfg.Logging.GetMaxBackups(),
		Compress:   cfg.Logging.Compress,
	}

	// 文件编码器（JSON格式）
	fileEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})

	consoleEncoderCfg := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    encodeLevelColor,
		EncodeTime:     encodeTimeColor,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	fileLevel := parseLevel(cfg.Logging.GetLevel())
	consoleLevel := parseLevel(cfg.Logging.GetConsoleLevel())

	fileCore := zapcore.NewCore(fileEncoder, zapcore.AddSync(fw), fileLevel)

	var consoleCore zapcore.Core
	if cfg.Logging.GetColorize() {
		consoleEncoder := newMarkdownConsoleEncoder(consoleEncoderCfg, true)
		consoleCore = zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), consoleLevel)
	} else {
		consoleEncoder := newMarkdownConsoleEncoder(consoleEncoderCfg, false)
		consoleCore = zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), consoleLevel)
	}

	// 组合多个Core（同时输出到文件和控制台）
	core := zapcore.NewTee(fileCore, consoleCore)

	// 创建Logger
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.Fields(
			zap.String("logger", name),
		),
	)

	return logger, logger.Sugar(), nil
}

// parseLevel 解析日志级别字符串
func parseLevel(levelStr string) zapcore.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "dpanic":
		return zapcore.DPanicLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// encodeLevelColor 带颜色的级别编码器（控制台）
func encodeLevelColor(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch l {
	case zapcore.DebugLevel:
		enc.AppendString("\033[35mDEBUG\033[0m") // 紫色
	case zapcore.InfoLevel:
		enc.AppendString("\033[32mINFO\033[0m") // 绿色
	case zapcore.WarnLevel:
		enc.AppendString("\033[33mWARN\033[0m") // 黄色
	case zapcore.ErrorLevel:
		enc.AppendString("\033[31mERROR\033[0m") // 红色
	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		enc.AppendString("\033[31mFATAL\033[0m") // 红色
	default:
		enc.AppendString(l.String())
	}
}

// encodeTimeColor 带颜色和简化格式的时间编码器
func encodeTimeColor(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("15:04:05"))
}

// ShutdownLoggers 关闭所有Logger
func ShutdownLoggers() error {
	loggers := []*zap.Logger{
		GeneralLogger,
		SystemLogger,
		NetworkLogger,
		ProxyLogger,
		DebugLogger,
	}

	for _, logger := range loggers {
		if logger != nil {
			if err := logger.Sync(); err != nil {
				// 忽略标准输出同步错误
				errStr := err.Error()
				if !strings.Contains(errStr, "sync /dev/stdout") &&
					!strings.Contains(errStr, "invalid argument") &&
					!strings.Contains(errStr, "handle is invalid") {
					return err
				}
			}
		}
	}

	return nil
}
