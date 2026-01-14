# Robustness Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve system robustness through unified error handling, concurrency safety, logging, and resilience mechanisms.

**Architecture:** Layered approach - start with foundational error types, then add concurrency primitives, logging middleware, and finally resilience patterns (circuit breaker, retry, rate limiting).

**Tech Stack:** Go 1.21+, slog (stdlib), golang.org/x/time/rate, fsnotify (optional)

---

## Phase 1: Error Handling Foundation (P0)

### Task 1.1: Create Error Types Package

**Files:**
- Create: `pkg/errors/errors.go`
- Create: `pkg/errors/errors_test.go`

**Step 1: Write the failing test**

```go
// pkg/errors/errors_test.go
package errors

import (
	"errors"
	"testing"
)

func TestProxyError_Error(t *testing.T) {
	err := &ProxyError{
		Code:    ErrCodeUpstream,
		Message: "connection failed",
	}
	if err.Error() != "[UPSTREAM_ERROR] connection failed" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestProxyError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := &ProxyError{
		Code:    ErrCodeInternal,
		Message: "wrapped",
		Cause:   cause,
	}
	if !errors.Is(err, cause) {
		t.Error("Unwrap should return cause")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected bool
	}{
		{ErrCodeUpstream, true},
		{ErrCodeRateLimit, true},
		{ErrCodeConfig, false},
		{ErrCodeValidation, false},
	}
	for _, tt := range tests {
		err := &ProxyError{Code: tt.code}
		if IsRetryable(err) != tt.expected {
			t.Errorf("IsRetryable(%s) = %v, want %v", tt.code, !tt.expected, tt.expected)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/errors/... -v`
Expected: FAIL (package does not exist)

**Step 3: Write minimal implementation**

```go
// pkg/errors/errors.go
package errors

import "fmt"

type ErrorCode string

const (
	ErrCodeUpstream   ErrorCode = "UPSTREAM_ERROR"
	ErrCodeRateLimit  ErrorCode = "RATE_LIMIT"
	ErrCodeConfig     ErrorCode = "CONFIG_ERROR"
	ErrCodeInternal   ErrorCode = "INTERNAL_ERROR"
	ErrCodeValidation ErrorCode = "VALIDATION_ERROR"
	ErrCodeTimeout    ErrorCode = "TIMEOUT"
)

type ProxyError struct {
	Code    ErrorCode
	Message string
	Cause   error
	Context map[string]any
}

func (e *ProxyError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *ProxyError) Unwrap() error {
	return e.Cause
}

func New(code ErrorCode, message string) *ProxyError {
	return &ProxyError{Code: code, Message: message}
}

func Wrap(code ErrorCode, message string, cause error) *ProxyError {
	return &ProxyError{Code: code, Message: message, Cause: cause}
}

func IsRetryable(err error) bool {
	var pe *ProxyError
	if As(err, &pe) {
		switch pe.Code {
		case ErrCodeUpstream, ErrCodeRateLimit, ErrCodeTimeout:
			return true
		}
	}
	return false
}

// Re-export stdlib errors functions for convenience
var (
	Is     = fmt.Errorf // placeholder
	As     func(err error, target any) bool
	Join   func(errs ...error) error
)

func init() {
	// Use stdlib errors
	import_errors()
}
```

Wait - let me fix that. The re-export approach is cleaner:

```go
// pkg/errors/errors.go
package errors

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrCodeUpstream   ErrorCode = "UPSTREAM_ERROR"
	ErrCodeRateLimit  ErrorCode = "RATE_LIMIT"
	ErrCodeConfig     ErrorCode = "CONFIG_ERROR"
	ErrCodeInternal   ErrorCode = "INTERNAL_ERROR"
	ErrCodeValidation ErrorCode = "VALIDATION_ERROR"
	ErrCodeTimeout    ErrorCode = "TIMEOUT"
)

type ProxyError struct {
	Code    ErrorCode
	Message string
	Cause   error
	Context map[string]any
}

func (e *ProxyError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *ProxyError) Unwrap() error {
	return e.Cause
}

func New(code ErrorCode, message string) *ProxyError {
	return &ProxyError{Code: code, Message: message}
}

func Wrap(code ErrorCode, message string, cause error) *ProxyError {
	return &ProxyError{Code: code, Message: message, Cause: cause}
}

func WithContext(err *ProxyError, ctx map[string]any) *ProxyError {
	err.Context = ctx
	return err
}

func IsRetryable(err error) bool {
	var pe *ProxyError
	if errors.As(err, &pe) {
		switch pe.Code {
		case ErrCodeUpstream, ErrCodeRateLimit, ErrCodeTimeout:
			return true
		}
	}
	return false
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/errors/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/errors/
git commit -m "feat(errors): add unified ProxyError type with error codes"
```

---

### Task 1.2: Add Defer Error Helper

**Files:**
- Modify: `pkg/errors/errors.go`
- Modify: `pkg/errors/errors_test.go`

**Step 1: Write the failing test**

```go
// Add to pkg/errors/errors_test.go
func TestCloseWithError(t *testing.T) {
	var finalErr error
	
	// Simulate successful close
	CloseWithError(&finalErr, io.NopCloser(nil))
	if finalErr != nil {
		t.Errorf("expected nil error, got %v", finalErr)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/errors/... -v -run TestCloseWithError`
Expected: FAIL (undefined: CloseWithError)

**Step 3: Write minimal implementation**

```go
// Add to pkg/errors/errors.go
import "io"

// CloseWithError safely closes a closer and captures any error
func CloseWithError(errPtr *error, c io.Closer) {
	if cerr := c.Close(); cerr != nil && *errPtr == nil {
		*errPtr = cerr
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/errors/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/errors/
git commit -m "feat(errors): add CloseWithError helper for defer cleanup"
```

---

## Phase 2: Concurrency Safety (P0)

### Task 2.1: Create Safe Map Type

**Files:**
- Create: `pkg/sync/safemap.go`
- Create: `pkg/sync/safemap_test.go`

**Step 1: Write the failing test**

```go
// pkg/sync/safemap_test.go
package sync

import (
	"sync"
	"testing"
)

func TestSafeMap_GetSet(t *testing.T) {
	m := NewSafeMap[string, int]()
	m.Set("key", 42)
	
	val, ok := m.Get("key")
	if !ok || val != 42 {
		t.Errorf("Get() = %v, %v; want 42, true", val, ok)
	}
}

func TestSafeMap_Concurrent(t *testing.T) {
	m := NewSafeMap[int, int]()
	var wg sync.WaitGroup
	
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			m.Set(n, n*2)
			m.Get(n)
		}(i)
	}
	wg.Wait()
	
	if m.Len() != 100 {
		t.Errorf("Len() = %d; want 100", m.Len())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/sync/... -v`
Expected: FAIL (package does not exist)

**Step 3: Write minimal implementation**

```go
// pkg/sync/safemap.go
package sync

import "sync"

type SafeMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{m: make(map[K]V)}
}

func (s *SafeMap[K, V]) Get(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	return v, ok
}

func (s *SafeMap[K, V]) Set(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

func (s *SafeMap[K, V]) Delete(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}

func (s *SafeMap[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/sync/... -v -race`
Expected: PASS (no race conditions)

**Step 5: Commit**

```bash
git add pkg/sync/
git commit -m "feat(sync): add generic SafeMap for concurrent access"
```

---

### Task 2.2: Create Worker Pool with Graceful Shutdown

**Files:**
- Create: `pkg/sync/worker.go`
- Create: `pkg/sync/worker_test.go`

**Step 1: Write the failing test**

```go
// pkg/sync/worker_test.go
package sync

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPool_Stop(t *testing.T) {
	var count atomic.Int32
	wp := NewWorkerPool(2)
	
	for i := 0; i < 5; i++ {
		wp.Submit(func() {
			count.Add(1)
			time.Sleep(10 * time.Millisecond)
		})
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	
	if err := wp.Stop(ctx); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
	
	if count.Load() != 5 {
		t.Errorf("count = %d; want 5", count.Load())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/sync/... -v -run TestWorkerPool`
Expected: FAIL (undefined: NewWorkerPool)

**Step 3: Write minimal implementation**

```go
// pkg/sync/worker.go
package sync

import (
	"context"
	"sync"
)

type WorkerPool struct {
	tasks chan func()
	wg    sync.WaitGroup
	done  chan struct{}
}

func NewWorkerPool(workers int) *WorkerPool {
	wp := &WorkerPool{
		tasks: make(chan func(), 100),
		done:  make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
	return wp
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.done:
			// Drain remaining tasks
			for task := range wp.tasks {
				task()
			}
			return
		case task, ok := <-wp.tasks:
			if !ok {
				return
			}
			task()
		}
	}
}

func (wp *WorkerPool) Submit(task func()) {
	select {
	case <-wp.done:
		return
	case wp.tasks <- task:
	}
}

func (wp *WorkerPool) Stop(ctx context.Context) error {
	close(wp.done)
	close(wp.tasks)
	
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/sync/... -v -race`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/sync/
git commit -m "feat(sync): add WorkerPool with graceful shutdown"
```

---

## Phase 3: Logging & Monitoring (P1)

### Task 3.1: Create Request ID Middleware

**Files:**
- Create: `pkg/middleware/requestid.go`
- Create: `pkg/middleware/requestid_test.go`

**Step 1: Write the failing test**

```go
// pkg/middleware/requestid_test.go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		if id == "" {
			t.Error("request ID should not be empty")
		}
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("response should have X-Request-ID header")
	}
}

func TestRequestIDMiddleware_PreserveExisting(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		if id != "existing-id" {
			t.Errorf("request ID = %s; want existing-id", id)
		}
	}))
	
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "existing-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/middleware/... -v`
Expected: FAIL (package does not exist)

**Step 3: Write minimal implementation**

```go
// pkg/middleware/requestid.go
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateID()
		}
		
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/middleware/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/middleware/
git commit -m "feat(middleware): add request ID middleware for tracing"
```

---

### Task 3.2: Add Sensitive Data Sanitizer

**Files:**
- Create: `pkg/log/sanitize.go`
- Create: `pkg/log/sanitize_test.go`

**Step 1: Write the failing test**

```go
// pkg/log/sanitize_test.go
package log

import "testing"

func TestSanitize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sk-abc123def456", "sk-***"},
		{"Bearer sk-secret123", "Bearer sk-***"},
		{"api_key=sk-test", "api_key=sk-***"},
		{"normal text", "normal text"},
	}
	
	for _, tt := range tests {
		result := Sanitize(tt.input)
		if result != tt.expected {
			t.Errorf("Sanitize(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/log/... -v`
Expected: FAIL (package does not exist)

**Step 3: Write minimal implementation**

```go
// pkg/log/sanitize.go
package log

import "regexp"

var apiKeyPattern = regexp.MustCompile(`sk-[a-zA-Z0-9]+`)

func Sanitize(s string) string {
	return apiKeyPattern.ReplaceAllString(s, "sk-***")
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/log/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/log/
git commit -m "feat(log): add sensitive data sanitizer"
```

---

## Phase 4: Resilience Mechanisms (P1)

### Task 4.1: Create Circuit Breaker

**Files:**
- Create: `pkg/resilience/circuitbreaker.go`
- Create: `pkg/resilience/circuitbreaker_test.go`

**Step 1: Write the failing test**

```go
// pkg/resilience/circuitbreaker_test.go
package resilience

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)
	testErr := errors.New("fail")
	
	// Fail 3 times
	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return testErr })
	}
	
	// Should be open now
	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_ResetsAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)
	testErr := errors.New("fail")
	
	// Open the circuit
	cb.Execute(func() error { return testErr })
	cb.Execute(func() error { return testErr })
	
	// Wait for reset
	time.Sleep(60 * time.Millisecond)
	
	// Should allow one request (half-open)
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Errorf("expected nil after reset, got %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/resilience/... -v`
Expected: FAIL (package does not exist)

**Step 3: Write minimal implementation**

```go
// pkg/resilience/circuitbreaker.go
package resilience

import (
	"errors"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type state int

const (
	stateClosed state = iota
	stateOpen
	stateHalfOpen
)

type CircuitBreaker struct {
	mu          sync.Mutex
	state       state
	failures    int
	threshold   int
	resetAfter  time.Duration
	lastFailure time.Time
}

func NewCircuitBreaker(threshold int, resetAfter time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:  threshold,
		resetAfter: resetAfter,
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	
	if cb.state == stateOpen {
		if time.Since(cb.lastFailure) > cb.resetAfter {
			cb.state = stateHalfOpen
		} else {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
	}
	cb.mu.Unlock()
	
	err := fn()
	
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()
		if cb.failures >= cb.threshold {
			cb.state = stateOpen
		}
		return err
	}
	
	cb.failures = 0
	cb.state = stateClosed
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/resilience/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/resilience/
git commit -m "feat(resilience): add circuit breaker implementation"
```

---

### Task 4.2: Add Retry with Exponential Backoff

**Files:**
- Modify: `pkg/resilience/retry.go`
- Modify: `pkg/resilience/retry_test.go`

**Step 1: Write the failing test**

```go
// pkg/resilience/retry_test.go
package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryWithBackoff_Success(t *testing.T) {
	attempts := 0
	err := RetryWithBackoff(context.Background(), 3, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary")
		}
		return nil
	})
	
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d; want 2", attempts)
	}
}

func TestRetryWithBackoff_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	err := RetryWithBackoff(ctx, 3, func() error {
		return errors.New("fail")
	})
	
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/resilience/... -v -run TestRetry`
Expected: FAIL (undefined: RetryWithBackoff)

**Step 3: Write minimal implementation**

```go
// pkg/resilience/retry.go
package resilience

import (
	"context"
	"time"
)

func RetryWithBackoff(ctx context.Context, maxRetries int, fn func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if err = fn(); err == nil {
			return nil
		}
		
		if i < maxRetries-1 {
			backoff := time.Duration(1<<i) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/resilience/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/resilience/
git commit -m "feat(resilience): add retry with exponential backoff"
```

---

## Phase 5: Configuration Validation (P1)

### Task 5.1: Add Config Validation

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/config_test.go` (if not exists)

**Step 1: Write the failing test**

```go
// internal/config/config_test.go
package config

import "testing"

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name:    "invalid port",
			cfg:     &Config{Server: ServerConfig{Port: -1}},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

**Step 2-5:** Implementation depends on existing config structure. Review `internal/config/config.go` first.

---

## Phase 6: Integration (P2)

### Task 6.1: Integrate Error Types into Handlers

**Files:**
- Modify: `internal/handler/*.go`

Apply `ProxyError` wrapping to all error returns. Example pattern:

```go
// Before
return fmt.Errorf("upstream failed: %w", err)

// After
return errors.Wrap(errors.ErrCodeUpstream, "upstream failed", err)
```

### Task 6.2: Add Circuit Breaker to Provider Calls

**Files:**
- Modify: `internal/provider/*.go`

Wrap provider calls with circuit breaker:

```go
cb := resilience.NewCircuitBreaker(5, 30*time.Second)
err := cb.Execute(func() error {
    return provider.Call(ctx, req)
})
```

---

## Verification Checklist

After all tasks complete:

- [ ] `go test ./... -race` passes
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` has no warnings
- [ ] Manual test: start server, send requests, verify logging

---

## Summary

| Phase | Tasks | Est. Time |
|-------|-------|-----------|
| 1. Error Handling | 1.1, 1.2 | 1 hour |
| 2. Concurrency | 2.1, 2.2 | 1.5 hours |
| 3. Logging | 3.1, 3.2 | 1 hour |
| 4. Resilience | 4.1, 4.2 | 1.5 hours |
| 5. Config | 5.1 | 0.5 hours |
| 6. Integration | 6.1, 6.2 | 2 hours |

**Total: ~7.5 hours**
