package crypto

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
)

func TestConfigWarmupDoesNotBlockConcurrentCallers(t *testing.T) {
	loader := newAppJSConfigLoader()
	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	load := func() xxteaConfig {
		calls.Add(1)
		<-release
		return xxteaConfig{Key: fallbackKey, Version: fallbackVer, Identifier: identifier}
	}
	var callers sync.WaitGroup
	for range 32 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			loader.start(load)
		}()
	}
	started := make(chan struct{})
	go func() {
		callers.Wait()
		close(started)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("warmup callers blocked on the shared download")
	}
	if loader.cfg.Load() != nil {
		t.Fatal("published configuration before the download completed")
	}
	if calls.Load() > 1 {
		t.Fatalf("started %d downloads, want at most one", calls.Load())
	}
}

func TestConfigWaitCancellationDoesNotInterruptOtherCallers(t *testing.T) {
	loader := newAppJSConfigLoader()
	release := make(chan struct{})
	var calls atomic.Int32
	want := xxteaConfig{Key: fallbackKey, Version: "test-version", Identifier: "test-id"}
	loader.start(func() xxteaConfig {
		calls.Add(1)
		<-release
		return want
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := loader.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled waiter returned %v", err)
	}
	select {
	case <-loader.ready:
		t.Fatal("one cancelled waiter completed shared initialization")
	default:
	}
	close(release)
	deadline, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	if err := loader.wait(deadline); err != nil {
		t.Fatal(err)
	}
	if got := loader.cfg.Load(); got == nil || *got != want {
		t.Fatalf("got snapshot %v, want %v", got, want)
	}
	loader.start(func() xxteaConfig {
		calls.Add(1)
		return xxteaConfig{}
	})
	if calls.Load() != 1 {
		t.Fatalf("started %d downloads, want one", calls.Load())
	}
}

type appJSClientFunc func(*fhttp.Request) (*fhttp.Response, error)

func (f appJSClientFunc) Do(req *fhttp.Request) (*fhttp.Response, error) {
	return f(req)
}

func TestDownloadAppJSReturnsNetworkFailureWithoutWaitingForDeadline(t *testing.T) {
	wantErr := errors.New("connection refused")
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := downloadAppJS(ctx, appJSClientFunc(func(req *fhttp.Request) (*fhttp.Response, error) {
			return nil, wantErr
		}), nil)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, wantErr) {
			t.Fatalf("got %v, want the network error", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("network failure waited for the download deadline")
	}
}

type contextBody struct {
	ctx    context.Context
	closed bool
}

func (b *contextBody) Read([]byte) (int, error) {
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (b *contextBody) Close() error {
	b.closed = true
	return nil
}

func TestDownloadAppJSDeadlineCoversResponseBody(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	body := &contextBody{}
	_, err := downloadAppJS(ctx, appJSClientFunc(func(req *fhttp.Request) (*fhttp.Response, error) {
		body.ctx = req.Context()
		return &fhttp.Response{StatusCode: fhttp.StatusOK, Body: body}, nil
	}), nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want deadline exceeded", err)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestDownloadAppJSRejectsErrorResponses(t *testing.T) {
	js, err := downloadAppJS(context.Background(), appJSClientFunc(func(req *fhttp.Request) (*fhttp.Response, error) {
		return &fhttp.Response{
			StatusCode: fhttp.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("upstream unavailable")),
		}, nil
	}), nil)
	if err == nil || js != "" {
		t.Fatalf("accepted an HTTP error page: js=%q, err=%v", js, err)
	}
}
