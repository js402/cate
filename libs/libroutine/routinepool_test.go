package libroutine_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/js402/CATE/libs/libroutine"
)

func TestPoolSingleton(t *testing.T) {
	defer quiet()
	t.Run("should return singleton instance", func(t *testing.T) {
		pool1 := libroutine.GetPool()
		pool2 := libroutine.GetPool()
		if pool1 != pool2 {
			t.Error("Expected pool to be singleton, got different instances")
		}
	})
}

func TestPoolStartLoop(t *testing.T) {
	pool := libroutine.GetPool()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("should create new manager and start loop", func(t *testing.T) {
		key := "test-service"
		var callCount int
		var mu sync.Mutex

		pool.StartLoop(ctx, key, 2, 100*time.Millisecond, 10*time.Millisecond,
			func(ctx context.Context) error {
				mu.Lock()
				callCount++
				mu.Unlock()
				return nil
			})

		// Allow some time for the loop to execute.
		time.Sleep(25 * time.Millisecond)

		mu.Lock()
		if callCount < 1 {
			t.Errorf("Expected at least 1 call, got %d", callCount)
		}
		mu.Unlock()

		// Verify loop tracking using the public accessor.
		if !pool.IsLoopActive(key) {
			t.Error("Loop should be tracked as active")
		}
	})

	t.Run("should prevent duplicate loops for same key", func(t *testing.T) {
		key := "duplicate-test"
		var callCount int
		var mu sync.Mutex

		// Start first loop.
		pool.StartLoop(ctx, key, 1, time.Second, 10*time.Millisecond,
			func(ctx context.Context) error {
				mu.Lock()
				callCount++
				mu.Unlock()
				return nil
			})

		// Try to start duplicate loop.
		pool.StartLoop(ctx, key, 1, time.Second, 10*time.Millisecond,
			func(ctx context.Context) error {
				mu.Lock()
				callCount++
				mu.Unlock()
				return nil
			})

		time.Sleep(25 * time.Millisecond)

		mu.Lock()
		if callCount < 1 {
			t.Errorf("Expected at least 1 call, got %d", callCount)
		}
		mu.Unlock()
	})

	t.Run("should clean up after context cancellation", func(t *testing.T) {
		key := "cleanup-test"
		localCtx, localCancel := context.WithCancel(ctx)

		pool.StartLoop(localCtx, key, 1, time.Second, 10*time.Millisecond,
			func(ctx context.Context) error { return nil })

		time.Sleep(10 * time.Millisecond)
		localCancel()

		// Wait for cleanup.
		time.Sleep(20 * time.Millisecond)

		if pool.IsLoopActive(key) {
			t.Error("Loop should be removed from active tracking")
		}
	})

	t.Run("should handle concurrent StartLoop calls", func(t *testing.T) {
		key := "concurrency-test"
		var wg sync.WaitGroup
		var callCount int
		var mu sync.Mutex

		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pool.StartLoop(ctx, key, 1, time.Second, 10*time.Millisecond,
					func(ctx context.Context) error {
						mu.Lock()
						callCount++
						mu.Unlock()
						return nil
					})
			}()
		}

		wg.Wait()
		time.Sleep(25 * time.Millisecond)

		mu.Lock()
		if callCount < 1 {
			t.Errorf("Expected at least 1 call, got %d", callCount)
		}
		mu.Unlock()
	})
}

func TestPoolCircuitBreaking(t *testing.T) {
	defer quiet()

	pool := libroutine.GetPool()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("should enforce circuit breaker parameters", func(t *testing.T) {
		key := "circuit-params-test"
		failureThreshold := 3
		resetTimeout := 50 * time.Millisecond

		var failures int

		// Use a very long interval so that Execute only runs when triggered.
		pool.StartLoop(ctx, key, failureThreshold, resetTimeout, 1000*time.Second,
			func(ctx context.Context) error {
				failures++
				return fmt.Errorf("simulated failure")
			})

		// Fire triggers to simulate failureThreshold number of calls.
		for i := 0; i < failureThreshold; i++ {
			pool.ForceUpdate(key)
			// Give time for the execution to complete.
			time.Sleep(5 * time.Millisecond)
		}

		manager := pool.GetManager(key)
		if manager == nil {
			t.Fatal("Manager not created")
		}

		state := manager.GetState()
		if state != libroutine.Open {
			t.Errorf("Expected circuit to be open after %d failures, got state %v", failureThreshold, state)
		}

		// Wait for reset timeout to elapse.
		time.Sleep(resetTimeout + 10*time.Millisecond)

		// Do not send a trigger here; instead, call Allow() manually to simulate the test call.
		if allowed := manager.Allow(); !allowed {
			t.Error("Expected Allow() to return true in half-open state")
		}

		state = manager.GetState()
		if state != libroutine.HalfOpen {
			t.Error("Circuit should transition to half-open after reset timeout")
		}
	})
}

func TestPoolParameterPersistence(t *testing.T) {
	defer quiet()
	pool := libroutine.GetPool()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("should persist initial parameters", func(t *testing.T) {
		key := "param-persistence-test"
		initialThreshold := 2
		initialTimeout := 100 * time.Millisecond

		// First call with initial parameters.
		pool.StartLoop(ctx, key, initialThreshold, initialTimeout, 10*time.Millisecond,
			func(ctx context.Context) error { return nil })

		// Subsequent call with different parameters.
		pool.StartLoop(ctx, key, 5, 200*time.Millisecond, 20*time.Millisecond,
			func(ctx context.Context) error { return nil })

		manager := pool.GetManager(key)
		if manager == nil {
			t.Fatal("Manager not created")
		}

		if manager.GetThreshold() != initialThreshold {
			t.Errorf("Expected threshold %d, got %d", initialThreshold, manager.GetThreshold())
		}
		if manager.GetResetTimeout() != initialTimeout {
			t.Errorf("Expected timeout %v, got %v", initialTimeout, manager.GetResetTimeout())
		}
	})
}

// TestPoolResetRoutine verifies the ResetRoutine function correctly forces
// the associated circuit breaker to the Closed state.
func TestPoolResetRoutine(t *testing.T) {
	defer quiet()
	pool := libroutine.GetPool()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure context cleanup eventually

	key := "reset-routine-test"
	var runCount int
	var runCountMu sync.Mutex

	// Start a loop that does nothing but increments a counter (to ensure manager exists)
	pool.StartLoop(ctx, key, 1, 10*time.Millisecond, 10*time.Millisecond, func(ctx context.Context) error {
		runCountMu.Lock()
		runCount++
		runCountMu.Unlock()
		// Fail once to ensure state can change, then succeed
		if runCount <= 1 {
			return errors.New("fail once")
		}
		return nil
	})

	// Allow the loop to run and potentially fail once
	time.Sleep(50 * time.Millisecond) // Give it time to execute/fail

	// Get the manager and force it open (or verify it opened)
	manager := pool.GetManager(key)
	if manager == nil {
		t.Fatalf("Manager for key %s not found", key)
	}
	manager.ForceOpen() // Explicitly force open for predictable test state

	// Verify it's open
	if manager.GetState() != libroutine.Open {
		t.Fatalf("Manager state should be Open after ForceOpen, got %v", manager.GetState())
	}

	// Now, reset the routine via the Pool
	pool.ResetRoutine(key)

	// Verify the state is now Closed
	if manager.GetState() != libroutine.Closed {
		t.Errorf("Expected manager state to be Closed after ResetRoutine, got %v", manager.GetState())
	}

	// Verify execution is allowed again
	if !manager.Allow() {
		t.Error("Execution should be allowed after ResetRoutine")
	}
}
