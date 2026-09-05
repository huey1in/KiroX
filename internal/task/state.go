package task

import (
	"context"
	"sync"
	"time"
)

// State 任务状态（从原 App 脱离为独立单例）
type State struct {
	mu        sync.Mutex
	running   bool
	batch     *taskBatch
	total     int
	completed int
	success   int
	failed    int
	results   []map[string]interface{}
	startTime time.Time
	logs      []string
	logsMu    sync.Mutex
}

type taskBatch struct {
	ctx    context.Context
	cancel context.CancelFunc
}

var Manager = &State{
	logs: make([]string, 0),
}

// beginBatchLocked publishes cancellation before the background worker starts.
func (s *State) beginBatchLocked(count int) *taskBatch {
	ctx, cancel := context.WithCancel(context.Background())
	batch := &taskBatch{ctx: ctx, cancel: cancel}
	s.batch = batch
	s.running = true
	s.total = count
	s.completed = 0
	s.success = 0
	s.failed = 0
	s.results = nil
	s.startTime = time.Now()
	return batch
}

func (s *State) stopBatch() bool {
	s.mu.Lock()
	if !s.running || s.batch == nil {
		s.mu.Unlock()
		return false
	}
	batch := s.batch
	s.mu.Unlock()
	batch.cancel()
	return true
}

func (s *State) finishBatch(batch *taskBatch) {
	batch.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.batch == batch {
		s.running = false
		s.batch = nil
	}
}

// AppendLog 追加日志，最多保留 500 条
func (s *State) AppendLog(msg string) {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	s.logs = append(s.logs, msg)
	if len(s.logs) > 500 {
		s.logs = s.logs[len(s.logs)-500:]
	}
}

// GetLogs 获取所有当前日志记录的副本
func (s *State) GetLogs() []string {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()
	logs := make([]string, len(s.logs))
	copy(logs, s.logs)
	return logs
}

// GetStatus 获取当前并发状态 (结构与之前 GetStatus() map 保持一致)
func (s *State) GetStatus() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	elapsed := 0.0
	if s.running && !s.startTime.IsZero() {
		elapsed = time.Since(s.startTime).Seconds()
	}

	return map[string]interface{}{
		"running":   s.running,
		"total":     s.total,
		"completed": s.completed,
		"success":   s.success,
		"failed":    s.failed,
		"elapsed":   elapsed,
	}
}
