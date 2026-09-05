package task

import (
	"context"
	"sync"
	"testing"
)

func beginTestBatch(s *State, count int) *taskBatch {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.beginBatchLocked(count)
}

func TestBatchCanStopBeforeWorkerStarts(t *testing.T) {
	s := &State{}
	batch := beginTestBatch(s, 3)
	defer s.finishBatch(batch)
	if !s.stopBatch() {
		t.Fatal("new batch was not stoppable")
	}
	if batch.ctx.Err() != context.Canceled {
		t.Fatalf("batch context error = %v", batch.ctx.Err())
	}
	if !s.GetStatus()["running"].(bool) {
		t.Fatal("batch became idle before its workers finished")
	}
	s.finishBatch(batch)
	if s.GetStatus()["running"].(bool) {
		t.Fatal("finished batch is still running")
	}
	if s.stopBatch() {
		t.Fatal("idle state accepted a stop request")
	}
}

func TestFinishedBatchCannotStopReplacement(t *testing.T) {
	s := &State{}
	oldBatch := beginTestBatch(s, 2)
	s.finishBatch(oldBatch)
	newBatch := beginTestBatch(s, 5)
	defer s.finishBatch(newBatch)

	oldBatch.cancel()
	s.finishBatch(oldBatch)
	if newBatch.ctx.Err() != nil {
		t.Fatalf("old batch canceled replacement: %v", newBatch.ctx.Err())
	}
	status := s.GetStatus()
	if !status["running"].(bool) || status["total"] != 5 {
		t.Fatalf("old batch changed replacement state: %v", status)
	}
	if !s.stopBatch() || newBatch.ctx.Err() != context.Canceled {
		t.Fatal("replacement lost its cancellation handle")
	}
}

func TestConcurrentStopRequests(t *testing.T) {
	s := &State{}
	batch := beginTestBatch(s, 10)
	defer s.finishBatch(batch)
	var callers sync.WaitGroup
	for i := 0; i < 20; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			if !s.stopBatch() {
				t.Error("active batch rejected stop request")
			}
		}()
	}
	callers.Wait()
	if batch.ctx.Err() != context.Canceled {
		t.Fatal("concurrent stop requests did not cancel batch")
	}
	if !s.GetStatus()["running"].(bool) {
		t.Fatal("stop released the batch before workers finished")
	}
}
