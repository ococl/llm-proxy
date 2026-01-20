package backend

import (
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 10*time.Second)

	if cb.GetState() != StateClosed {
		t.Errorf("Initial state = %v, want %v", cb.GetState(), StateClosed)
	}

	if !cb.IsAvailable() {
		t.Error("Circuit breaker should be available initially")
	}
}

func TestCircuitBreaker_FailureThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 10*time.Second)

	for i := 0; i < 3; i++ {
		cb.Call(func() error {
			return &CircuitOpenError{}
		})
	}

	if cb.GetState() != StateOpen {
		t.Errorf("State after 3 failures = %v, want %v", cb.GetState(), StateOpen)
	}

	err := cb.Call(func() error { return nil })
	if err != ErrCircuitOpen {
		t.Errorf("Call on open circuit = %v, want %v", err, ErrCircuitOpen)
	}
}

func TestCircuitBreaker_HalfOpenTransition(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

	for i := 0; i < 3; i++ {
		cb.Call(func() error {
			return &CircuitOpenError{}
		})
	}

	if cb.GetState() != StateOpen {
		t.Fatal("Circuit should be open")
	}

	time.Sleep(150 * time.Millisecond)

	cb.mu.Lock()
	if cb.state != StateOpen {
		t.Error("State should still be open before any call")
	}
	cb.mu.Unlock()

	cb.Call(func() error { return nil })

	if cb.GetState() != StateHalfOpen {
		t.Errorf("State after timeout = %v, want %v", cb.GetState(), StateHalfOpen)
	}
}

func TestCircuitBreaker_SuccessRecovery(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

	for i := 0; i < 3; i++ {
		cb.Call(func() error {
			return &CircuitOpenError{}
		})
	}

	time.Sleep(150 * time.Millisecond)

	for i := 0; i < 2; i++ {
		cb.Call(func() error { return nil })
	}

	if cb.GetState() != StateClosed {
		t.Errorf("State after recovery = %v, want %v", cb.GetState(), StateClosed)
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

	for i := 0; i < 3; i++ {
		cb.Call(func() error {
			return &CircuitOpenError{}
		})
	}

	time.Sleep(150 * time.Millisecond)

	cb.Call(func() error {
		return &CircuitOpenError{}
	})

	if cb.GetState() != StateOpen {
		t.Errorf("State after half-open failure = %v, want %v", cb.GetState(), StateOpen)
	}
}

func TestCircuitBreakerManager(t *testing.T) {
	mgr := NewCircuitBreakerManager(3, 2, 10*time.Second)

	breaker1 := mgr.GetBreaker("backend1")
	breaker2 := mgr.GetBreaker("backend2")

	if breaker1 == breaker2 {
		t.Error("Different backends should have different circuit breakers")
	}

	breaker1_again := mgr.GetBreaker("backend1")
	if breaker1 != breaker1_again {
		t.Error("Same backend should return same circuit breaker instance")
	}
}

func TestCircuitBreakerManager_Concurrent(t *testing.T) {
	mgr := NewCircuitBreakerManager(5, 2, 10*time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "backend1"
			if idx%2 == 0 {
				key = "backend2"
			}
			breaker := mgr.GetBreaker(key)
			breaker.Call(func() error { return nil })
		}(i)
	}

	wg.Wait()

	if !mgr.IsAvailable("backend1") {
		t.Error("backend1 should be available")
	}
	if !mgr.IsAvailable("backend2") {
		t.Error("backend2 should be available")
	}
}
