package middleware

import (
	"context"
	"net/http"
	"sync/atomic"

	domainerror "llm-proxy/domain/error"
	"llm-proxy/domain/port"
)

type ConcurrencyLimiter struct {
	global       chan struct{}
	queueSize    int64
	maxQueue     int
	configGetter func() port.ConcurrencyConfig
}

func NewConcurrencyLimiter(configProvider port.ConfigProvider) *ConcurrencyLimiter {
	configGetter := func() port.ConcurrencyConfig {
		return configProvider.Get().Concurrency
	}
	cfg := configGetter()
	cl := &ConcurrencyLimiter{
		maxQueue:     cfg.MaxQueueSize,
		configGetter: configGetter,
	}
	if cfg.Enabled {
		cl.global = make(chan struct{}, cfg.MaxRequests)
	}
	return cl
}

func (cl *ConcurrencyLimiter) Acquire(ctx context.Context) error {
	cfg := cl.configGetter()
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
	cfg := cl.configGetter()
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
		cfg := cl.configGetter()
		if !cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), cfg.QueueTimeout)
		defer cancel()

		if err := cl.Acquire(ctx); err != nil {
			domainerror.WriteConcurrencyLimit(w)
			return
		}
		defer cl.Release()
		next.ServeHTTP(w, r)
	})
}
