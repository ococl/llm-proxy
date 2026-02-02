package logging

import (
	"sync"
	"time"

	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// asyncWriter 异步日志写入器
type asyncWriter struct {
	buffer     chan []byte
	dropOnFull bool
	wg         sync.WaitGroup
	stopCh     chan struct{}
	writer     zapcore.WriteSyncer
	mu         sync.RWMutex
}

// newAsyncWriter 创建异步写入器
func newAsyncWriter(writer zapcore.WriteSyncer, bufferSize int, dropOnFull bool) *asyncWriter {
	if bufferSize <= 0 {
		bufferSize = 10000
	}

	aw := &asyncWriter{
		buffer:     make(chan []byte, bufferSize),
		dropOnFull: dropOnFull,
		stopCh:     make(chan struct{}),
		writer:     writer,
	}

	aw.wg.Add(1)
	go aw.run()

	return aw
}

func (aw *asyncWriter) run() {
	defer aw.wg.Done()

	for {
		select {
		case data := <-aw.buffer:
			if data != nil {
				_, _ = aw.writer.Write(data)
			}
		case <-aw.stopCh:
			// 刷新剩余数据
			aw.flush()
			return
		}
	}
}

func (aw *asyncWriter) flush() {
	for {
		select {
		case data := <-aw.buffer:
			if data != nil {
				_, _ = aw.writer.Write(data)
			}
		default:
			return
		}
	}
}

func (aw *asyncWriter) Write(p []byte) (int, error) {
	aw.mu.RLock()
	defer aw.mu.RUnlock()

	select {
	case aw.buffer <- append([]byte(nil), p...):
		return len(p), nil
	default:
		if aw.dropOnFull {
			return 0, nil
		}
		// 阻塞等待
		aw.buffer <- append([]byte(nil), p...)
		return len(p), nil
	}
}

func (aw *asyncWriter) Sync() error {
	aw.flush()
	return aw.writer.Sync()
}

func (aw *asyncWriter) Stop() {
	close(aw.stopCh)
	aw.wg.Wait()
}

// timeAndSizeRotateLogger 支持时间和大小双策略的日志记录器
type timeAndSizeRotateLogger struct {
	*lumberjack.Logger
	timeStrategy  string
	lastRotate    time.Time
	mu            sync.RWMutex
	forceRotateCh chan struct{}
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// newTimeAndSizeRotateLogger 创建双策略日志记录器
func newTimeAndSizeRotateLogger(filename string, maxSize, maxAge, maxBackups int, compress bool, timeStrategy string) *timeAndSizeRotateLogger {
	tl := &timeAndSizeRotateLogger{
		Logger: &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    maxSize, // MB
			MaxAge:     maxAge,  // days
			MaxBackups: maxBackups,
			Compress:   compress,
		},
		timeStrategy:  timeStrategy,
		lastRotate:    time.Now(),
		forceRotateCh: make(chan struct{}, 1),
		stopCh:        make(chan struct{}),
	}

	// 启动时间轮转检查 goroutine
	if timeStrategy != "" && timeStrategy != "none" {
		tl.wg.Add(1)
		go tl.timeRotateChecker()
	}

	return tl
}

func (tl *timeAndSizeRotateLogger) timeRotateChecker() {
	defer tl.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if tl.shouldTimeRotate() {
				tl.Rotate()
			}
		case <-tl.forceRotateCh:
			tl.Rotate()
		case <-tl.stopCh:
			return
		}
	}
}

func (tl *timeAndSizeRotateLogger) shouldTimeRotate() bool {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	now := time.Now()

	switch tl.timeStrategy {
	case "daily":
		return now.Day() != tl.lastRotate.Day() || now.Month() != tl.lastRotate.Month() || now.Year() != tl.lastRotate.Year()
	case "hourly":
		return now.Hour() != tl.lastRotate.Hour() || now.Day() != tl.lastRotate.Day()
	default:
		return false
	}
}

func (tl *timeAndSizeRotateLogger) Write(p []byte) (int, error) {
	// 检查是否需要按时间轮转
	if tl.shouldTimeRotate() {
		select {
		case tl.forceRotateCh <- struct{}{}:
		default:
		}
	}

	return tl.Logger.Write(p)
}

func (tl *timeAndSizeRotateLogger) Rotate() error {
	tl.mu.Lock()
	tl.lastRotate = time.Now()
	tl.mu.Unlock()

	return tl.Logger.Rotate()
}

func (tl *timeAndSizeRotateLogger) Stop() {
	close(tl.stopCh)
	tl.wg.Wait()
}

// createRotateLogger 根据配置创建合适的日志记录器
func createRotateLogger(filename string, rotation RotationConfig, categoryMaxSize, categoryMaxAge int) *timeAndSizeRotateLogger {
	// 使用分类配置覆盖全局配置
	maxSize := rotation.MaxSizeMB
	if categoryMaxSize > 0 {
		maxSize = categoryMaxSize
	}
	if maxSize <= 0 {
		maxSize = 100
	}

	maxAge := rotation.MaxAgeDays
	if categoryMaxAge > 0 {
		maxAge = categoryMaxAge
	}
	if maxAge <= 0 {
		maxAge = 7
	}

	maxBackups := rotation.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 21
	}

	return newTimeAndSizeRotateLogger(
		filename,
		maxSize,
		maxAge,
		maxBackups,
		rotation.Compress,
		rotation.TimeStrategy,
	)
}

// RotationConfig 日志轮转配置
type RotationConfig struct {
	MaxSizeMB    int
	TimeStrategy string
	MaxAgeDays   int
	MaxBackups   int
	Compress     bool
}
