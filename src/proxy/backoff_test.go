package proxy

import (
	"testing"
	"time"
)

func TestBackoffStrategy_CalculateDelay(t *testing.T) {
	backoff := NewBackoffStrategy(
		100*time.Millisecond,
		5*time.Second,
		2.0,
		0.0,
		3,
	)

	tests := []struct {
		name         string
		attempt      int
		minExpected  time.Duration
		maxExpected  time.Duration
		shouldBeZero bool
	}{
		{
			name:         "attempt 0",
			attempt:      0,
			shouldBeZero: true,
		},
		{
			name:        "attempt 1",
			attempt:     1,
			minExpected: 100 * time.Millisecond,
			maxExpected: 100 * time.Millisecond,
		},
		{
			name:        "attempt 2",
			attempt:     2,
			minExpected: 200 * time.Millisecond,
			maxExpected: 200 * time.Millisecond,
		},
		{
			name:        "attempt 3",
			attempt:     3,
			minExpected: 400 * time.Millisecond,
			maxExpected: 400 * time.Millisecond,
		},
		{
			name:         "attempt 4 exceeds max retries",
			attempt:      4,
			shouldBeZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := backoff.CalculateDelay(tt.attempt)

			if tt.shouldBeZero {
				if delay != 0 {
					t.Errorf("CalculateDelay(%d) = %v, want 0", tt.attempt, delay)
				}
				return
			}

			if delay < tt.minExpected || delay > tt.maxExpected {
				t.Errorf("CalculateDelay(%d) = %v, want between %v and %v",
					tt.attempt, delay, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestBackoffStrategy_WithJitter(t *testing.T) {
	backoff := NewBackoffStrategy(
		100*time.Millisecond,
		5*time.Second,
		2.0,
		0.2,
		3,
	)

	baseDelay := 100 * time.Millisecond
	minJitter := time.Duration(float64(baseDelay) * 0.8)
	maxJitter := time.Duration(float64(baseDelay) * 1.2)

	delay := backoff.CalculateDelay(1)

	if delay < minJitter || delay > maxJitter {
		t.Errorf("CalculateDelay(1) with jitter = %v, want between %v and %v",
			delay, minJitter, maxJitter)
	}
}

func TestBackoffStrategy_MaxDelayCap(t *testing.T) {
	backoff := NewBackoffStrategy(
		1*time.Second,
		2*time.Second,
		2.0,
		0.0,
		10,
	)

	delay := backoff.CalculateDelay(5)

	if delay > 2*time.Second {
		t.Errorf("CalculateDelay(5) = %v, should be capped at 2s", delay)
	}
}

func TestBackoffStrategy_ShouldRetry(t *testing.T) {
	backoff := DefaultBackoffStrategy()

	tests := []struct {
		attempt int
		want    bool
	}{
		{0, false},
		{1, true},
		{2, true},
		{3, true},
		{4, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := backoff.ShouldRetry(tt.attempt)
			if got != tt.want {
				t.Errorf("ShouldRetry(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}
