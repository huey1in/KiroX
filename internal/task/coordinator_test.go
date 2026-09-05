package task

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func awaitTaskSignal[T any](t *testing.T, signal <-chan T) T {
	t.Helper()
	select {
	case value := <-signal:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("task did not respond before timeout")
		var zero T
		return zero
	}
}

func TestRunTasksRefillsAvailableWorkers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const count, concurrency = 9, 3
	started := make(chan int, count)
	fastPermits := make(chan struct{}, count)
	var active, peak atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		runTasks(ctx, count, concurrency, 0, func(index int) {
			n := active.Add(1)
			defer active.Add(-1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			started <- index
			if index == 0 {
				<-ctx.Done()
				return
			}
			select {
			case <-fastPermits:
			case <-ctx.Done():
			}
		})
	}()

	seen := make(map[int]bool)
	for i := 0; i < concurrency; i++ {
		seen[awaitTaskSignal(t, started)] = true
	}
	if active.Load() != concurrency {
		t.Fatalf("expected %d active tasks, got %d", concurrency, active.Load())
	}
	for i := 1; i < count; i++ {
		fastPermits <- struct{}{}
	}
	// All other tasks must start even while task zero still occupies one worker.
	for i := concurrency; i < count; i++ {
		seen[awaitTaskSignal(t, started)] = true
	}
	cancel()
	awaitTaskSignal(t, done)
	if len(seen) != count {
		t.Fatalf("expected each task once, got %v", seen)
	}
	if peak.Load() != concurrency {
		t.Fatalf("expected concurrency limit %d, got %d", concurrency, peak.Load())
	}
}

func TestRunTasksCancelsWhileWorkersAreOccupied(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const concurrency = 3
	started := make(chan int, 100)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runTasks(ctx, 100, concurrency, 0, func(index int) {
			started <- index
			<-ctx.Done()
		})
	}()
	for i := 0; i < concurrency; i++ {
		awaitTaskSignal(t, started)
	}
	cancel()
	awaitTaskSignal(t, done)
	if len(started) != 0 {
		t.Fatalf("started %d queued tasks after cancellation", len(started))
	}
}

func TestRunTasksCancelsSerialDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan int, 2)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runTasks(ctx, 2, 1, time.Hour, func(index int) { started <- index })
	}()
	if index := awaitTaskSignal(t, started); index != 0 {
		t.Fatalf("first task index = %d", index)
	}
	cancel()
	awaitTaskSignal(t, done)
	if len(started) != 0 {
		t.Fatal("started another task after cancellation")
	}
}

func TestRunTasksAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, concurrency := range []int{0, 1, 4} {
		runTasks(ctx, 10, concurrency, 0, func(index int) {
			t.Fatalf("started task %d with concurrency %d after cancellation", index, concurrency)
		})
	}
}
