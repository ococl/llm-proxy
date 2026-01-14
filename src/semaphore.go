package main

import (
	"context"
	"net/http"
	"sync/atomic"
)

type ConcurrencyLimiter struct {
	global    chan struct{}
	queueSize int64
	maxQueue  int
	configMgr *ConfigManager
}

func NewConcurrencyLimiter(configMgr *ConfigManager) *ConcurrencyLimiter {
	cfg := configMgr.Get().Concurrency
	cl := &ConcurrencyLimiter{
		configMgr: configMgr,
		maxQueue:  cfg.GetMaxQueueSize(),
	}
	if cfg.Enabled {
		cl.global = make(chan struct{}, cfg.GetMaxRequests())
	}
	return cl
}

func (cl *ConcurrencyLimiter) Acquire(ctx context.Context) error {
	cfg := cl.configMgr.Get().Concurrency
	if !cfg.Enabled || cl.global == nil {
		return nil
	}
	if int(atomic.LoadInt64(&cl.queueSize)) >= cl.maxQueue {
		return context.DeadlineExceeded
	}
	atomic.AddInt64(&cl.queueSize, 1)
	defer atomic.AddInt64(&cl.queueSize, -1)

	select {
	case cl.global <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cl *ConcurrencyLimiter) Release() {
	cfg := cl.configMgr.Get().Concurrency
	if !cfg.Enabled || cl.global == nil {
		return
	}
	select {
	case <-cl.global:
	default:
	}
}

func (cl *ConcurrencyLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := cl.configMgr.Get().Concurrency
		if !cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), cfg.GetQueueTimeout())
		defer cancel()

		if err := cl.Acquire(ctx); err != nil {
			WriteJSONError(w, ErrConcurrencyLimit, http.StatusServiceUnavailable, "")
			return
		}
		defer cl.Release()
		next.ServeHTTP(w, r)
	})
}
