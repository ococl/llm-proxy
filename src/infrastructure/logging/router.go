package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"llm-proxy/infrastructure/config"
)

// safeJoin 安全地连接 baseDir 和 relPath，防止路径逃逸攻击。
// 如果 relPath 尝试跳出 baseDir，返回错误。
func safeJoin(baseDir, relPath string) (string, error) {
	// 如果 relPath 是绝对路径，直接拒绝（除非它等于 baseDir）
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("拒绝绝对路径: %s", relPath)
	}

	// 清理路径并获取规范路径
	fullPath := filepath.Join(baseDir, relPath)
	cleanPath := filepath.Clean(fullPath)

	// 验证清理后的路径仍然在 baseDir 内
	// 使用 strings.HasPrefix 需要确保 baseDir 以目录分隔符结尾
	baseDirClean := filepath.Clean(baseDir)
	if !strings.HasPrefix(cleanPath, baseDirClean) {
		// 再次检查边界情况（如 baseDir 是 cleanPath 的前缀但不是目录边界）
		if cleanPath != baseDirClean && !strings.HasPrefix(cleanPath, baseDirClean+string(filepath.Separator)) {
			return "", fmt.Errorf("路径逃逸尝试: %s -> %s", relPath, cleanPath)
		}
	}

	return cleanPath, nil
}

// TargetLogger 为每个输出目标维护独立的 logger 实例
type TargetLogger struct {
	consoleLogger *zap.Logger
	fileLogger    *zap.Logger
	consoleLevel  zapcore.Level
	fileLevel     zapcore.Level
	mu            sync.RWMutex
}

// MultiTargetRouter 多目标日志路由器
type MultiTargetRouter struct {
	config      *MultiTargetConfig
	loggers     map[LogCategory]*TargetLogger
	initialized bool
	mu          sync.RWMutex
}

var (
	globalRouter *MultiTargetRouter
	routerMu     sync.RWMutex
)

// InitMultiTargetRouter 初始化多目标日志路由器
func InitMultiTargetRouter(cfg *config.Config) error {
	routerMu.Lock()
	defer routerMu.Unlock()

	multiCfg := ConvertFromLegacyConfig(cfg)
	// baseDir 已经在 Init 函数中包含了日期目录，这里直接使用
	// 避免：./logs/2026-01-26/2026-01-26/ 双层日期目录问题

	router := &MultiTargetRouter{
		config:  multiCfg,
		loggers: make(map[LogCategory]*TargetLogger),
	}

	// 创建各分类的日志器
	for categoryStr, targetCfg := range multiCfg.Categories {
		category := LogCategory(categoryStr)
		if err := router.createLoggerForCategory(category, &targetCfg); err != nil {
			return fmt.Errorf("创建分类[%s]日志器失败: %w", category, err)
		}
	}

	// 初始化全局日志器
	if err := router.initGlobalLoggers(cfg); err != nil {
		return fmt.Errorf("初始化全局日志器失败: %w", err)
	}

	globalRouter = router
	return nil
}

// createLoggerForCategory 为指定分类创建日志器
func (r *MultiTargetRouter) createLoggerForCategory(category LogCategory, cfg *TargetConfig) error {
	if category == LogCategoryRequestBody {
		return nil
	}

	// 继承全局配置中的设置
	consoleEnabled := r.config.Console.Enabled
	fileEnabled := r.config.File.Enabled

	// 根据目标配置调整
	switch LogTarget(cfg.Target) {
	case LogTargetConsole:
		fileEnabled = false
	case LogTargetFile:
		consoleEnabled = false
	case LogTargetNone:
		consoleEnabled = false
		fileEnabled = false
	}

	logger := &TargetLogger{
		consoleLevel: parseLevel(cfg.Levels.Console),
		fileLevel:    parseLevel(cfg.Levels.File),
	}

	// 创建控制台日志器
	if consoleEnabled {
		consoleLogger, err := r.createConsoleLogger(category, cfg)
		if err != nil {
			return fmt.Errorf("创建控制台日志器失败: %w", err)
		}
		logger.consoleLogger = consoleLogger
	}

	// 创建文件日志器
	if fileEnabled {
		fileLogger, err := r.createFileLogger(category, cfg)
		if err != nil {
			return fmt.Errorf("创建文件日志器失败: %w", err)
		}
		logger.fileLogger = fileLogger
	}

	r.mu.Lock()
	r.loggers[category] = logger
	r.mu.Unlock()

	return nil
}

// createConsoleLogger 创建控制台日志器
func (r *MultiTargetRouter) createConsoleLogger(category LogCategory, cfg *TargetConfig) (*zap.Logger, error) {
	encoderConfig := zapcore.EncoderConfig{
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

	// 使用 markdown 控制台编码器
	consoleEncoder := newMarkdownConsoleEncoder(
		encoderConfig,
		r.config.Console.Colorize,
		r.config.Console.Style,
	)

	level := parseLevel(r.config.Console.Level)
	if cfg != nil && cfg.Levels.Console != "" {
		level = parseLevel(cfg.Levels.Console)
	}

	core := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	return zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.Fields(zap.String("category", string(category))),
	), nil
}

// createFileLogger 创建文件日志器
func (r *MultiTargetRouter) createFileLogger(category LogCategory, cfg *TargetConfig) (*zap.Logger, error) {
	// 确定文件路径
	filePath := r.getLogFilePath(category, cfg)

	// 创建目录
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 确定文件大小限制
	maxSizeMB := r.config.File.MaxSizeMB
	maxAge := r.config.File.MaxAgeDays
	maxBackups := r.config.File.MaxBackups
	compress := r.config.File.Compress

	if cfg != nil {
		if cfg.MaxSize > 0 {
			maxSizeMB = cfg.MaxSize
		}
		if cfg.MaxAge > 0 {
			maxAge = cfg.MaxAge
		}
		if cfg.Compress {
			compress = true
		}
	}

	// 创建 lumberjack 写入器
	writer := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    maxSizeMB,
		MaxAge:     maxAge,
		MaxBackups: maxBackups,
		Compress:   compress,
	}

	// 创建文件编码器
	encoderConfig := zapcore.EncoderConfig{
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

	var fileEncoder zapcore.Encoder
	if r.config.File.Format == "text" {
		fileEncoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		fileEncoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	level := parseLevel(r.config.File.Level)
	if cfg != nil && cfg.Levels.File != "" {
		level = parseLevel(cfg.Levels.File)
	}

	core := zapcore.NewCore(
		fileEncoder,
		&syncWriteSyncer{w: zapcore.AddSync(writer)},
		level,
	)

	return zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.Fields(zap.String("category", string(category))),
	), nil
}

// getLogFilePath 获取日志文件路径
func (r *MultiTargetRouter) getLogFilePath(category LogCategory, cfg *TargetConfig) string {
	baseDir := r.config.File.BaseDir

	if cfg != nil && cfg.Path != "" {
		// 相对路径：相对于 baseDir，使用 safeJoin 防止路径逃逸
		if !filepath.IsAbs(cfg.Path) {
			safePath, err := safeJoin(baseDir, cfg.Path)
			if err != nil {
				// 如果 safeJoin 失败，回退到默认路径
				fmt.Fprintf(os.Stderr, "safeJoin failed for category %s: %v, falling back to default\n", category, err)
				return filepath.Join(baseDir, "general.log")
			}
			return safePath
		}
		// 绝对路径：安全检查后使用
		safePath, err := safeJoin(filepath.Dir(cfg.Path), filepath.Base(cfg.Path))
		if err != nil {
			fmt.Fprintf(os.Stderr, "safeJoin failed for absolute path %s: %v, falling back to default\n", cfg.Path, err)
			return filepath.Join(baseDir, "general.log")
		}
		return safePath
	}

	// 默认路径
	switch category {
	case LogCategoryGeneral:
		return filepath.Join(baseDir, "general.log")
	case LogCategoryRequest:
		return filepath.Join(baseDir, "requests", "requests.log")
	case LogCategoryError:
		return filepath.Join(baseDir, "errors", "errors.log")
	case LogCategoryDebug:
		return filepath.Join(baseDir, "debug", "debug.log")
	case LogCategoryNetwork:
		return filepath.Join(baseDir, "network", "network.log")
	default:
		return filepath.Join(baseDir, "general.log")
	}
}

// initGlobalLoggers 初始化全局日志器引用
func (r *MultiTargetRouter) initGlobalLoggers(cfg *config.Config) error {
	// 为向后兼容，初始化全局日志器
	r.mu.RLock()
	if logger, ok := r.loggers[LogCategoryGeneral]; ok {
		if logger.fileLogger != nil {
			GeneralLogger = logger.fileLogger
			GeneralSugar = logger.fileLogger.Sugar()
		}
	}
	r.mu.RUnlock()

	return nil
}

// Log 输出日志到指定分类
func (r *MultiTargetRouter) Log(category LogCategory, level zapcore.Level, msg string, fields ...zap.Field) {
	r.mu.RLock()
	logger, ok := r.loggers[category]
	r.mu.RUnlock()

	if !ok {
		return
	}

	logger.mu.RLock()
	defer logger.mu.RUnlock()

	// 根据级别判断是否输出到控制台和文件
	if logger.consoleLogger != nil && level >= logger.consoleLevel {
		logger.consoleLogger.Log(level, msg, fields...)
	}

	if logger.fileLogger != nil && level >= logger.fileLevel {
		logger.fileLogger.Log(level, msg, fields...)
	}
}

// GetLogger 获取指定分类的日志器（用于向后兼容）
func (r *MultiTargetRouter) GetLogger(category LogCategory) *zap.Logger {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if logger, ok := r.loggers[category]; ok {
		if logger.fileLogger != nil {
			return logger.fileLogger
		}
	}
	return nil
}

// GetSugar 获取指定分类的 Sugar logger
func (r *MultiTargetRouter) GetSugar(category LogCategory) *zap.SugaredLogger {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if logger, ok := r.loggers[category]; ok {
		if logger.fileLogger != nil {
			return logger.fileLogger.Sugar()
		}
	}
	return nil
}

// Shutdown 关闭所有日志器
func (r *MultiTargetRouter) Shutdown() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []error
	for _, logger := range r.loggers {
		logger.mu.Lock()
		if logger.consoleLogger != nil {
			if err := logger.consoleLogger.Sync(); err != nil {
				if !isIgnorableSyncError(err) {
					errs = append(errs, fmt.Errorf("控制台日志器同步失败: %w", err))
				}
			}
		}
		if logger.fileLogger != nil {
			if err := logger.fileLogger.Sync(); err != nil {
				if !isIgnorableSyncError(err) {
					errs = append(errs, fmt.Errorf("文件日志器同步失败: %w", err))
				}
			}
		}
		logger.mu.Unlock()
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// GetGlobalRouter 获取全局路由器
func GetGlobalRouter() *MultiTargetRouter {
	routerMu.RLock()
	defer routerMu.RUnlock()
	return globalRouter
}

// isIgnorableSyncError 判断是否为可忽略的同步错误
func isIgnorableSyncError(err error) bool {
	errStr := err.Error()
	ignoredPatterns := []string{
		"sync /dev/stdout",
		"invalid argument",
		"handle is invalid",
		"bad file descriptor",
	}
	for _, pattern := range ignoredPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// LogRequestCategory 便捷函数：记录请求日志
func LogRequestCategory(level zapcore.Level, msg string, fields ...zap.Field) {
	if router := GetGlobalRouter(); router != nil {
		router.Log(LogCategoryRequest, level, msg, fields...)
	}
}

// LogErrorCategory 便捷函数：记录错误日志
func LogErrorCategory(level zapcore.Level, msg string, fields ...zap.Field) {
	if router := GetGlobalRouter(); router != nil {
		router.Log(LogCategoryError, level, msg, fields...)
	}
}

// LogNetworkCategory 便捷函数：记录网络日志
func LogNetworkCategory(level zapcore.Level, msg string, fields ...zap.Field) {
	if router := GetGlobalRouter(); router != nil {
		router.Log(LogCategoryNetwork, level, msg, fields...)
	}
}

// LogDebugCategory 便捷函数：记录调试日志
func LogDebugCategory(level zapcore.Level, msg string, fields ...zap.Field) {
	if router := GetGlobalRouter(); router != nil {
		router.Log(LogCategoryDebug, level, msg, fields...)
	}
}

// LogGeneralCategory 便捷函数：记录普通日志
func LogGeneralCategory(level zapcore.Level, msg string, fields ...zap.Field) {
	if router := GetGlobalRouter(); router != nil {
		router.Log(LogCategoryGeneral, level, msg, fields...)
	}
}
