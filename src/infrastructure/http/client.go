package http

import (
	"net"
	"net/http"
	"time"
)

type ClientConfig struct {
	ConnectTimeout        time.Duration
	ResponseHeaderTimeout time.Duration
	TotalTimeout          time.Duration
	MaxConnsPerHost       int
	MaxIdleConns          int
	KeepAlive             time.Duration
	IdleConnTimeout       time.Duration
}

func NewHTTPClient(cfg ClientConfig) *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   cfg.ConnectTimeout,
			KeepAlive: cfg.KeepAlive,
		}).DialContext,
		TLSHandshakeTimeout:   cfg.ConnectTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxConnsPerHost / 4,
	}

	return &http.Client{
		Timeout:   cfg.TotalTimeout,
		Transport: transport,
	}
}

func DefaultClientConfig(backendCount int) ClientConfig {
	maxConns := backendCount * 5
	if maxConns < 10 {
		maxConns = 10
	}
	if maxConns > 50 {
		maxConns = 50
	}

	return ClientConfig{
		ConnectTimeout:        10 * time.Second,
		ResponseHeaderTimeout: 3 * time.Minute,
		TotalTimeout:          15 * time.Minute,
		MaxConnsPerHost:       maxConns,
		MaxIdleConns:          20,
		KeepAlive:             5 * time.Minute,
		IdleConnTimeout:       10 * time.Minute,
	}
}
