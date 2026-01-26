package logging

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"llm-proxy/domain/port"
	"llm-proxy/infrastructure/config"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	requestLogger  *zap.SugaredLogger
	errorLogger    *zap.SugaredLogger
	metricsEnabled bool
	loggingCfg     *config.Logging
)

// newLumberjackSyncWriter 创建一个带 Sync 的 WriteSyncer，用于解决 Windows 文件锁定问题。
// lumberjack 在进行日志轮转时会尝试重命名文件，如果文件句柄仍被持有会导致错误。
// 通过包装 WriteSyncer 并在每次写入后调用 Sync()，可以确保文件缓冲被及时刷新。
func newLumberjackSyncWriter(lb *lumberjack.Logger) zapcore.WriteSyncer {
	return &syncWriteSyncer{w: zapcore.AddSync(lb)}
}

// syncWriteSyncer 包装 WriteSyncer，在每次写入后调用 Sync。
// 用于解决 Windows 上文件被锁定无法重命名的问题。
type syncWriteSyncer struct {
	w zapcore.WriteSyncer
}

func (s *syncWriteSyncer) Write(p []byte) (n int, err error) {
	n, err = s.w.Write(p)
	if err != nil {
		return n, err
	}
	// 写入后立即同步，释放文件句柄
	if err := s.w.Sync(); err != nil {
		// 忽略同步错误，因为文件可能已被关闭或正在轮转
		return n, nil
	}
	return n, nil
}

func (s *syncWriteSyncer) Sync() error {
	return s.w.Sync()
}

type markdownConsoleEncoder struct {
	zapcore.Encoder
	colored      bool
	consoleStyle string
}

var reqIDPattern = regexp.MustCompile(`\[req_[a-zA-Z0-9]+\]`)

type maskingCore struct {
	zapcore.Core
	masker *SensitiveDataMasker
}

func (c *maskingCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	entry.Message = c.masker.Mask(entry.Message)
	for i := range fields {
		if fields[i].Type == zapcore.StringType {
			fields[i].String = c.masker.Mask(fields[i].String)
		}
	}
	return c.Core.Write(entry, fields)
}

func (c *maskingCore) With(fields []zapcore.Field) zapcore.Core {
	return &maskingCore{Core: c.Core.With(fields), masker: c.masker}
}

func (c *maskingCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

func newMarkdownConsoleEncoder(cfg zapcore.EncoderConfig, colored bool, consoleStyle string) zapcore.Encoder {
	return &markdownConsoleEncoder{
		Encoder:      zapcore.NewConsoleEncoder(cfg),
		colored:      colored,
		consoleStyle: consoleStyle,
	}
}

func (enc *markdownConsoleEncoder) Clone() zapcore.Encoder {
	return &markdownConsoleEncoder{
		Encoder:      enc.Encoder.Clone(),
		colored:      enc.colored,
		consoleStyle: enc.consoleStyle,
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
	line.WriteString(" | ")

	levelStr := entry.Level.CapitalString()
	levelFormatted := fmt.Sprintf("%5s", levelStr)
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
		line.WriteString(levelFormatted)
		line.WriteString("\033[0m")
	} else {
		line.WriteString(levelFormatted)
	}
	line.WriteString(" | ")

	var reqID, model, backendModel, backend string
	filteredFields := make([]zapcore.Field, 0, len(fields))
	for _, field := range fields {
		switch field.Key {
		case "req_id":
			if field.Type == zapcore.StringType {
				reqID = field.String
			}
		case "model":
			if field.Type == zapcore.StringType {
				model = field.String
			}
		case "backend_model":
			if field.Type == zapcore.StringType {
				backendModel = field.String
			}
		case "backend":
			if field.Type == zapcore.StringType {
				backend = field.String
			}
		case "logger":
		default:
			filteredFields = append(filteredFields, field)
		}
	}

	if reqID != "" {
		colorMgr := GetGlobalColorManager()
		reqColor := colorMgr.GetRequestColor(reqID)
		if enc.colored && reqColor != "" {
			line.WriteString(reqColor)
			line.WriteString(reqID)
			line.WriteString("\033[0m")
		} else {
			line.WriteString(reqID)
		}
	}
	line.WriteString(" | ")

	msg := entry.Message
	line.WriteString(msg)

	if model != "" || backendModel != "" || backend != "" {
		colorMgr := GetGlobalColorManager()
		reqColor := colorMgr.GetRequestColor(reqID)
		if enc.colored && reqColor != "" {
			line.WriteString(" [backend=")
			line.WriteString(reqColor)
			line.WriteString(backend)
			if backendModel != "" {
				line.WriteString("\033[0m")
				line.WriteString(", model=")
				line.WriteString(reqColor)
				line.WriteString(backendModel)
			}
			line.WriteString("\033[0m")
			line.WriteString("]")
		} else {
			line.WriteString(" [backend=")
			line.WriteString(backend)
			if backendModel != "" {
				line.WriteString(", model=")
				line.WriteString(backendModel)
			}
			line.WriteString("]")
		}
	}

	if len(filteredFields) > 0 {
		enc.encodeFields(line, filteredFields)
	}

	line.WriteString("\n")

	buf := buffer.NewPool().Get()
	buf.Write(line.Bytes())
	return buf, nil
}

func (enc *markdownConsoleEncoder) encodeFields(line *bytes.Buffer, fields []zapcore.Field) {
	if enc.consoleStyle == "compact" {
		line.WriteString(" [")
		firstField := true
		for _, field := range fields {
			if field.Key == "logger" {
				continue
			}
			if !firstField {
				line.WriteString(", ")
			}
			firstField = false
			enc.writeField(line, field)
		}
		line.WriteString("]")
	} else {
		for _, field := range fields {
			if field.Key == "logger" {
				continue
			}
			line.WriteString("\n  ")
			enc.writeField(line, field)
		}
	}
}

func (enc *markdownConsoleEncoder) writeField(line *bytes.Buffer, field zapcore.Field) {
	if enc.colored {
		line.WriteString("\033[90m")
		line.WriteString(field.Key)
		line.WriteString("\033[0m=\033[33m")
		line.WriteString(fieldValueString(field))
		line.WriteString("\033[0m")
	} else {
		line.WriteString(field.Key)
		line.WriteString("=")
		line.WriteString(fieldValueString(field))
	}
}

func fieldValueString(field zapcore.Field) string {
	switch field.Type {
	case zapcore.StringType:
		return field.String
	case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type,
		zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type:
		return fmt.Sprintf("%d", field.Integer)
	case zapcore.BoolType:
		if field.Integer == 1 {
			return "true"
		}
		return "false"
	default:
		return field.String
	}
}

func Init(cfg *config.Config) error {
	baseDir := cfg.Logging.GetBaseDir()

	// 日志直接写入 logs/ 目录，不创建日期子目录
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("创建日志根目录失败 %s: %w", baseDir, err)
	}

	if cfg.Logging.ShouldUseDetailedMasking() {
		updateSensitivePatterns()
	}

	var err error

	generalLogPath := cfg.Logging.GeneralFile
	if generalLogPath == "" {
		generalLogPath = filepath.Join(baseDir, "general.log")
	}

	if cfg.Logging.SeparateFiles {
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
		if cfg.Logging.DebugMode {
			DebugLogger, DebugSugar, err = createLogger(cfg, "debug", filepath.Join(baseDir, "llm_debug", "debug.log"))
			if err != nil {
				return fmt.Errorf("初始化DebugLogger失败: %w", err)
			}
		} else {
			DebugLogger, DebugSugar = createNoOpLogger()
		}
	} else {
		GeneralLogger, GeneralSugar, err = createLogger(cfg, "general", generalLogPath)
		if err != nil {
			return fmt.Errorf("初始化GeneralLogger失败: %w", err)
		}
		SystemLogger = GeneralLogger.With(zap.String("category", "system"))
		SystemSugar = SystemLogger.Sugar()
		NetworkLogger = GeneralLogger.With(zap.String("category", "network"))
		NetworkSugar = NetworkLogger.Sugar()
		ProxyLogger = GeneralLogger.With(zap.String("category", "proxy"))
		ProxySugar = ProxyLogger.Sugar()
		if cfg.Logging.DebugMode {
			DebugLogger = GeneralLogger.With(zap.String("category", "debug"))
			DebugSugar = DebugLogger.Sugar()
		} else {
			DebugLogger, DebugSugar = createNoOpLogger()
		}
	}

	FileOnlyLogger, FileOnlySugar, err = createFileOnlyLogger(cfg, "fileonly", filepath.Join(baseDir, "proxy", "verbose.log"))
	if err != nil {
		return fmt.Errorf("初始化FileOnlyLogger失败: %w", err)
	}

	// 旧版 request/error 日志已废弃，多目标日志系统使用 categories 配置
	// 保留向后兼容但不推荐使用
	if err := initRequestErrorLoggers(cfg); err != nil {
		return err
	}

	if cfg.Logging.EnableMetrics {
		metricsEnabled = true
	}
	loggingCfg = &cfg.Logging

	startFlushTicker(cfg.Logging.GetFlushInterval())

	// 初始化新的多目标日志路由器
	if err := InitMultiTargetRouter(cfg); err != nil {
		// 多目标日志路由器初始化失败不应阻止应用启动
		// 日志将回退到传统的单一日志器模式
		return nil
	}

	return nil
}

func initRequestErrorLoggers(cfg *config.Config) error {
	baseDir := cfg.Logging.GetBaseDir()

	// 旧版 request/error 日志直接写入 logs/ 目录，不创建日期子目录
	reqDir := cfg.Logging.RequestDir
	if reqDir == "" {
		reqDir = filepath.Join(baseDir, "requests")
	}
	errDir := cfg.Logging.ErrorDir
	if errDir == "" {
		errDir = filepath.Join(baseDir, "errors")
	}

	if cfg.Logging.SeparateFiles {
		if err := os.MkdirAll(reqDir, 0755); err != nil {
			return fmt.Errorf("创建请求日志目录失败: %w", err)
		}
		if err := os.MkdirAll(errDir, 0755); err != nil {
			return fmt.Errorf("创建错误日志目录失败: %w", err)
		}
	}

	var err error
	RequestLogger, requestLogger, err = createFileOnlyLogger(cfg, "request", filepath.Join(reqDir, "requests.log"))
	if err != nil {
		return fmt.Errorf("初始化RequestLogger失败: %w", err)
	}
	ErrorLogger, errorLogger, err = createFileOnlyLogger(cfg, "error", filepath.Join(errDir, "errors.log"))
	if err != nil {
		return fmt.Errorf("初始化ErrorLogger失败: %w", err)
	}
	return nil
}

func startFlushTicker(interval int) {
	if interval <= 0 {
		return
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
	loggers := []*zap.Logger{GeneralLogger, SystemLogger, NetworkLogger, ProxyLogger, DebugLogger, FileOnlyLogger, RequestLogger, ErrorLogger}
	for _, logger := range loggers {
		if logger != nil {
			_ = logger.Sync()
		}
	}
}

func updateSensitivePatterns() {
	extraPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)([a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,})`),
		regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),
		regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		regexp.MustCompile(`\b(\+?\d[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`),
	}
	SensitivePatterns = append(SensitivePatterns, extraPatterns...)
}

func createLogger(cfg *config.Config, name, filePath string) (*zap.Logger, *zap.SugaredLogger, error) {
	fw := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    cfg.Logging.GetMaxFileSizeMB(),
		MaxAge:     cfg.Logging.GetMaxAgeDays(),
		MaxBackups: cfg.Logging.GetMaxBackups(),
		Compress:   cfg.Logging.Compress,
	}

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

	var fileEncoder zapcore.Encoder
	if cfg.Logging.GetFormat() == "text" {
		fileEncoder = zapcore.NewConsoleEncoder(fileEncoderCfg)
	} else {
		fileEncoder = zapcore.NewJSONEncoder(fileEncoderCfg)
	}

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

	// 使用带 Sync 的 WriteSyncer，解决 Windows 文件锁定问题
	fileCore := zapcore.NewCore(fileEncoder, newLumberjackSyncWriter(fw), fileLevel)

	var consoleCore zapcore.Core
	if cfg.Logging.GetColorize() {
		if cfg.Logging.GetConsoleFormat() == "plain" {
			consoleCore = zapcore.NewCore(zapcore.NewConsoleEncoder(consoleEncoderCfg), zapcore.AddSync(os.Stdout), consoleLevel)
		} else {
			consoleEncoder := newMarkdownConsoleEncoder(consoleEncoderCfg, true, cfg.Logging.GetConsoleStyle())
			consoleCore = zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), consoleLevel)
		}
	} else {
		if cfg.Logging.GetConsoleFormat() == "plain" {
			consoleCore = zapcore.NewCore(zapcore.NewConsoleEncoder(consoleEncoderCfg), zapcore.AddSync(os.Stdout), consoleLevel)
		} else {
			consoleEncoder := newMarkdownConsoleEncoder(consoleEncoderCfg, false, cfg.Logging.GetConsoleStyle())
			consoleCore = zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), consoleLevel)
		}
	}

	var core zapcore.Core
	teeCore := zapcore.NewTee(fileCore, consoleCore)
	if cfg.Logging.ShouldMaskSensitive() {
		core = &maskingCore{Core: teeCore, masker: NewSensitiveDataMasker()}
	} else {
		core = teeCore
	}

	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.Fields(zap.String("logger", name)),
	)

	return logger, logger.Sugar(), nil
}

func createNoOpLogger() (*zap.Logger, *zap.SugaredLogger) {
	nopCore := zapcore.NewNopCore()
	logger := zap.New(nopCore)
	return logger, logger.Sugar()
}

func createFileOnlyLogger(cfg *config.Config, name, filePath string) (*zap.Logger, *zap.SugaredLogger, error) {
	fw := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    cfg.Logging.GetMaxFileSizeMB(),
		MaxAge:     cfg.Logging.GetMaxAgeDays(),
		MaxBackups: cfg.Logging.GetMaxBackups(),
		Compress:   cfg.Logging.Compress,
	}

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

	var fileEncoder zapcore.Encoder
	if cfg.Logging.GetFormat() == "text" {
		fileEncoder = zapcore.NewConsoleEncoder(fileEncoderCfg)
	} else {
		fileEncoder = zapcore.NewJSONEncoder(fileEncoderCfg)
	}

	fileLevel := parseLevel(cfg.Logging.GetLevel())
	fileCore := zapcore.NewCore(fileEncoder, zapcore.AddSync(fw), fileLevel)

	var core zapcore.Core
	if cfg.Logging.ShouldMaskSensitive() {
		core = &maskingCore{Core: fileCore, masker: NewSensitiveDataMasker()}
	} else {
		core = fileCore
	}

	logger := zap.New(core, zap.AddCaller(), zap.Fields(zap.String("logger", name)))
	return logger, logger.Sugar(), nil
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
	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		enc.AppendString("\033[31mFATAL\033[0m")
	default:
		enc.AppendString(l.String())
	}
}

func encodeTimeColor(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("15:04:05"))
}

func Shutdown() error {
	if flushTicker != nil {
		flushTicker.Stop()
		close(flushDone)
		flushTicker = nil
		flushDone = nil
	}

	loggers := []*zap.Logger{GeneralLogger, SystemLogger, NetworkLogger, ProxyLogger, DebugLogger, RequestLogger, ErrorLogger}
	for _, logger := range loggers {
		if logger != nil {
			if err := logger.Sync(); err != nil {
				errStr := err.Error()
				if !strings.Contains(errStr, "sync /dev/stdout") &&
					!strings.Contains(errStr, "invalid argument") &&
					!strings.Contains(errStr, "handle is invalid") {
					return err
				}
			}
		}
	}

	// 关闭多目标日志路由器
	if router := GetGlobalRouter(); router != nil {
		if err := router.Shutdown(); err != nil {
			return err
		}
	}

	return nil
}

func WriteRequestLog(reqID, content string) {
	if requestLogger == nil {
		return
	}
	requestLogger.Infow("请求日志",
		port.ReqID(reqID),
		port.Content(content),
	)
}

func WriteErrorLog(reqID, content string) {
	if errorLogger == nil {
		return
	}
	errorLogger.Errorw("错误日志",
		port.ReqID(reqID),
		port.Content(content),
	)
}

func LogMetrics(reqID, modelAlias, status, finalBackend string, attempts int, totalLatencyMs int64, backendDetails string) {
	if !metricsEnabled || GeneralSugar == nil {
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
