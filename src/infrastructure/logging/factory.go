package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"llm-proxy/domain/port"
	"llm-proxy/infrastructure/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 全局日志变量
var (
	GeneralLogger  *zap.Logger
	GeneralSugar   *zap.SugaredLogger
	SystemLogger   *zap.Logger
	SystemSugar    *zap.SugaredLogger
	NetworkLogger  *zap.Logger
	NetworkSugar   *zap.SugaredLogger
	ProxyLogger    *zap.Logger
	ProxySugar     *zap.SugaredLogger
	DebugLogger    *zap.Logger
	DebugSugar     *zap.SugaredLogger
	FileOnlyLogger *zap.Logger
	FileOnlySugar  *zap.SugaredLogger
	RequestLogger  *zap.Logger
	errorLogger    *zap.SugaredLogger

	metricsEnabled bool
	loggingCfg     *config.Logging
	asyncWriters   []*asyncWriter
	rotateLoggers  []*timeAndSizeRotateLogger

	flushTicker *time.Ticker
	flushDone   chan struct{}
)

// 脱敏模式
var (
	configMu      sync.RWMutex
	testMode      = false
	maskSensitive = true
)

func SetTestMode(enabled bool) {
	testMode = enabled
}

func InitTestLoggers() {
	GeneralLogger, GeneralSugar = createNoOpLogger()
	SystemLogger, SystemSugar = createNoOpLogger()
	NetworkLogger, NetworkSugar = createNoOpLogger()
	ProxyLogger, ProxySugar = createNoOpLogger()
	DebugLogger, DebugSugar = createNoOpLogger()
	FileOnlyLogger, FileOnlySugar = createNoOpLogger()
}

func InitLogger(cfg *config.Config) error {
	configMu.Lock()
	maskSensitive = cfg.Logging.MaskSensitive
	configMu.Unlock()
	return Init(cfg)
}

func ShutdownLogger() {
	Shutdown()
}

// Init 初始化日志系统
func Init(cfg *config.Config) error {
	baseDir := cfg.Logging.GetBaseDir()

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("创建日志根目录失败 %s: %w", baseDir, err)
	}

	loggingCfg = &cfg.Logging

	// 初始化各分类日志
	categories := []string{"general", "system", "network", "proxy", "debug", "request", "error"}
	for _, cat := range categories {
		if err := initCategoryLogger(cfg, cat, baseDir); err != nil {
			return fmt.Errorf("初始化 %s 日志失败: %w", cat, err)
		}
	}

	// 启动异步刷新 ticker
	if cfg.Logging.IsAsyncEnabled() {
		startFlushTicker(cfg.Logging.GetAsyncFlushInterval())
	}

	return nil
}

func initCategoryLogger(cfg *config.Config, category, baseDir string) error {
	catCfg, exists := cfg.Logging.Categories[category]
	if !exists {
		// 使用默认配置
		catCfg = config.CategoryConfig{
			Level:  "info",
			Target: "both",
			Path:   category + ".log",
		}
	}

	// 检查是否禁用（level 为 none）
	if catCfg.GetLevel() == "none" {
		return nil
	}

	filePath := filepath.Join(baseDir, catCfg.Path)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 创建日志记录器
	logger, sugar, err := createCategoryLogger(cfg, category, filePath, catCfg)
	if err != nil {
		return err
	}

	// 赋值给全局变量
	switch category {
	case "general":
		GeneralLogger, GeneralSugar = logger, sugar
	case "system":
		SystemLogger, SystemSugar = logger, sugar
	case "network":
		NetworkLogger, NetworkSugar = logger, sugar
	case "proxy":
		ProxyLogger, ProxySugar = logger, sugar
	case "debug":
		DebugLogger, DebugSugar = logger, sugar
	case "request":
		RequestLogger = logger
	case "error":
		errorLogger = sugar
	}

	return nil
}

func createCategoryLogger(cfg *config.Config, category, filePath string, catCfg config.CategoryConfig) (*zap.Logger, *zap.SugaredLogger, error) {
	level := parseLevel(catCfg.GetLevel())
	target := catCfg.GetTarget()

	// 文件编码器配置
	fileEncoderCfg := zapcore.EncoderConfig{
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
	}

	var cores []zapcore.Core

	// 文件输出
	if target == "file" || target == "both" {
		rotation := RotationConfig{
			MaxSizeMB:    cfg.Logging.GetRotationMaxSizeMB(),
			TimeStrategy: cfg.Logging.GetRotationTimeStrategy(),
			MaxAgeDays:   cfg.Logging.GetRotationMaxAgeDays(),
			MaxBackups:   cfg.Logging.GetRotationMaxBackups(),
			Compress:     cfg.Logging.ShouldRotateCompress(),
		}

		rotateLogger := createRotateLogger(filePath, rotation, catCfg.MaxSizeMB, catCfg.MaxAgeDays)
		rotateLoggers = append(rotateLoggers, rotateLogger)

		var writer zapcore.WriteSyncer = zapcore.AddSync(rotateLogger)

		// 异步写入
		if cfg.Logging.IsAsyncEnabled() {
			asyncWriter := newAsyncWriter(writer, cfg.Logging.GetAsyncBufferSize(), cfg.Logging.ShouldDropOnFull())
			asyncWriters = append(asyncWriters, asyncWriter)
			writer = asyncWriter
		}

		fileEncoder := zapcore.NewJSONEncoder(fileEncoderCfg)
		fileCore := zapcore.NewCore(fileEncoder, writer, level)
		cores = append(cores, fileCore)
	}

	// 控制台输出
	if target == "console" || target == "both" {
		consoleEncoderCfg := fileEncoderCfg
		consoleEncoderCfg.EncodeLevel = encodeLevelColor
		consoleEncoderCfg.EncodeTime = encodeTimeColor
		consoleEncoder := newMarkdownConsoleEncoder(consoleEncoderCfg, true, "compact")
		consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level)
		cores = append(cores, consoleCore)
	}

	if len(cores) == 0 {
		return zap.NewNop(), zap.NewNop().Sugar(), nil
	}

	var core zapcore.Core
	if len(cores) == 1 {
		core = cores[0]
	} else {
		core = zapcore.NewTee(cores...)
	}

	// 脱敏处理
	if cfg.Logging.MaskSensitive {
		core = &maskingCore{Core: core, masker: NewSensitiveDataMasker()}
	}

	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.Fields(zap.String("category", category)),
	)

	return logger, logger.Sugar(), nil
}

func createNoOpLogger() (*zap.Logger, *zap.SugaredLogger) {
	nopCore := zapcore.NewNopCore()
	logger := zap.New(nopCore)
	return logger, logger.Sugar()
}

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
	case "none":
		return zapcore.FatalLevel + 1
	default:
		return zapcore.InfoLevel
	}
}

func encodeLevelColor(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	switch l {
	case zapcore.DebugLevel:
		enc.AppendString("\033[35mDEBUG\033[0m")
	case zapcore.InfoLevel:
		enc.AppendString("\033[32mINFO\033[0m")
	case zapcore.WarnLevel:
		enc.AppendString("\033[33mWARN\033[0m")
	case zapcore.ErrorLevel:
		enc.AppendString("\033[31mERROR\033[0m")
	default:
		enc.AppendString(l.String())
	}
}

func encodeTimeColor(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("15:04:05"))
}

func startFlushTicker(interval int) {
	if interval <= 0 {
		interval = 5
	}
	flushDone = make(chan struct{})
	flushTicker = time.NewTicker(time.Duration(interval) * time.Second)
	go func() {
		for {
			select {
			case <-flushTicker.C:
				syncAllLoggers()
			case <-flushDone:
				return
			}
		}
	}()
}

func syncAllLoggers() {
	loggers := []*zap.Logger{GeneralLogger, SystemLogger, NetworkLogger, ProxyLogger, DebugLogger, RequestLogger}
	for _, logger := range loggers {
		if logger != nil {
			_ = logger.Sync()
		}
	}
}

func Shutdown() error {
	if flushTicker != nil {
		flushTicker.Stop()
		close(flushDone)
	}

	for _, aw := range asyncWriters {
		aw.Stop()
	}

	for _, tl := range rotateLoggers {
		tl.Stop()
	}

	loggers := []*zap.Logger{GeneralLogger, SystemLogger, NetworkLogger, ProxyLogger, DebugLogger, RequestLogger}
	for _, logger := range loggers {
		if logger != nil {
			if err := logger.Sync(); err != nil {
				errStr := err.Error()
				if !strings.Contains(errStr, "sync /dev/stdout") &&
					!strings.Contains(errStr, "invalid argument") {
					return err
				}
			}
		}
	}

	return nil
}

func WriteRequestLog(reqID, content string) {
	if RequestLogger == nil {
		return
	}
	RequestLogger.Info("请求日志",
		zap.String("req_id", reqID),
		zap.String("content", content),
	)
}

func WriteErrorLog(reqID, content string) {
	if errorLogger == nil {
		return
	}
	errorLogger.Error("错误日志",
		zap.String("req_id", reqID),
		zap.String("content", content),
	)
}

func LogMetrics(reqID, modelAlias, status, finalBackend string, attempts int, totalLatencyMs int64, backendDetails string) {
	if GeneralSugar == nil {
		return
	}
	GeneralSugar.Infow("性能指标",
		port.ReqID(reqID),
		port.Model(modelAlias),
		port.Status(status),
		port.Backend(finalBackend),
		port.Attempt(attempts),
		port.DurationMSInt(totalLatencyMs),
		port.BackendDetails(backendDetails),
	)
}

func IsMetricsEnabled() bool {
	return metricsEnabled
}

func GetLoggingConfig() *config.Logging {
	return loggingCfg
}
