package email

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type emailRoundTripFunc func(*http.Request) (*http.Response, error)

func (f emailRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func awaitEmailResult[T any](t *testing.T, results <-chan T) T {
	t.Helper()
	select {
	case value := <-results:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("email operation did not respond before timeout")
		var zero T
		return zero
	}
}

func TestHTTPProviderCreationCancellation(t *testing.T) {
	responses := map[string]string{
		"/api/config":                `{"emailDomains":"mail.test"}`,
		"/api/emails/generate":       `{"id":"inbox","email":"box@mail.test"}`,
		"/api/emails/inbox":          `{"messages":[]}`,
		"/api/setting/websiteConfig": `{"code":200,"data":{"domainList":["mail.test"]}}`,
		"/api/public/genToken":       `{"code":200,"data":{"token":"test-token"}}`,
		"/api/public/addUser":        `{"code":200}`,
		"/api/public/emailList":      `{"code":200,"data":[]}`,
	}
	cases := []struct {
		name  string
		paths []string
		start func(context.Context, string) error
	}{
		{
			name:  "moemail",
			paths: []string{"/api/config", "/api/emails/generate", "/api/emails/inbox"},
			start: func(ctx context.Context, url string) error {
				_, err := NewMoeMailProviderContext(ctx, MoeMailConfig{URL: url}, "box", 3600000, "mail.test")
				return err
			},
		},
		{
			name:  "cloudmail",
			paths: []string{"/api/setting/websiteConfig", "/api/public/genToken", "/api/public/addUser", "/api/public/emailList"},
			start: func(ctx context.Context, url string) error {
				_, err := NewCloudMailProviderContext(ctx, CloudMailConfig{URL: url}, "box", "")
				return err
			},
		},
	}
	for _, tc := range cases {
		for _, blockedPath := range tc.paths {
			t.Run(tc.name+blockedPath, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				entered := make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == blockedPath {
						close(entered)
						<-ctx.Done()
						return
					}
					body, ok := responses[r.URL.Path]
					if !ok {
						t.Errorf("unexpected request: %s", r.URL.Path)
						w.WriteHeader(http.StatusNotFound)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, body)
				}))
				t.Cleanup(server.Close)
				result := make(chan error, 1)
				go func() { result <- tc.start(ctx, server.URL) }()
				awaitEmailResult(t, entered)
				cancel()
				if err := awaitEmailResult(t, result); !errors.Is(err, context.Canceled) {
					t.Fatalf("expected creation cancellation, got %v", err)
				}
			})
		}
	}
}

func TestMailNestAddressCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := NewMailNestProviderContext(ctx, MailNestConfig{})
	entered := make(chan struct{})
	provider.client.client.Transport = emailRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		close(entered)
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	result := make(chan error, 1)
	go func() {
		_, err := provider.GetAddress()
		result <- err
	}()
	awaitEmailResult(t, entered)
	cancel()
	if err := awaitEmailResult(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected address cancellation, got %v", err)
	}
}

func emailCodeWaiter(kind string, ctx context.Context, transport http.RoundTripper) func(int, int) (string, error) {
	client := &http.Client{Transport: transport}
	switch kind {
	case "moemail":
		provider := &MoeMailProvider{client: newMoeMailClient(ctx, MoeMailConfig{URL: "https://mail.test"}), emailID: "inbox"}
		provider.client.client = client
		return provider.WaitForCode
	case "cloudmail":
		provider := &CloudMailProvider{client: newCloudMailClient(ctx, CloudMailConfig{URL: "https://mail.test"})}
		provider.client.client = client
		provider.client.token = "test-token"
		return provider.WaitForCode
	default:
		provider := NewMailNestProviderContext(ctx, MailNestConfig{})
		provider.client.client = client
		return provider.WaitForCode
	}
}

func TestHTTPProviderPollingCancellation(t *testing.T) {
	for _, kind := range []string{"moemail", "cloudmail", "mailnest"} {
		for _, mode := range []string{"request", "empty", "no-code", "error"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				entered := make(chan struct{}, 1)
				transport := emailRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					entered <- struct{}{}
					if mode == "request" {
						<-req.Context().Done()
						return nil, req.Context().Err()
					}
					status := http.StatusOK
					body := `{"messages":[],"code":200,"data":[]}`
					if kind == "mailnest" {
						body = `{"code":"00000","data":[]}`
					}
					if mode == "no-code" {
						body = `{"messages":[{"subject":"hello"}],"code":200,"data":[{"emailId":1,"subject":"hello"}]}`
						if kind == "mailnest" {
							body = `{"code":"00000","data":[{"code_match":""}]}`
						}
					}
					if mode == "error" {
						status = http.StatusServiceUnavailable
					}
					return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
				})
				wait := emailCodeWaiter(kind, ctx, transport)
				result := make(chan error, 1)
				go func() {
					_, err := wait(3600, 60)
					result <- err
				}()
				awaitEmailResult(t, entered)
				if mode != "request" {
					// Allow the completed local response to reach the long polling delay.
					time.Sleep(10 * time.Millisecond)
				}
				cancel()
				if err := awaitEmailResult(t, result); !errors.Is(err, context.Canceled) {
					t.Fatalf("expected polling cancellation, got %v", err)
				}
			})
		}
	}
}

func TestHTTPProviderPollingStillReturnsCode(t *testing.T) {
	for _, kind := range []string{"moemail", "cloudmail", "mailnest"} {
		t.Run(kind, func(t *testing.T) {
			transport := emailRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				body := `{"messages":[{"content":"Code: 654321"}],"code":200,"data":[{"emailId":1,"text":"Code: 654321"}]}`
				if kind == "mailnest" {
					body = `{"code":"00000","data":[{"code_match":"654321"}]}`
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			})
			wait := emailCodeWaiter(kind, context.Background(), transport)
			code, err := wait(120, 0)
			if err != nil || code != "654321" {
				t.Fatalf("expected code 654321, got %q, %v", code, err)
			}
		})
	}
}

func TestWaitEmailPollCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- waitEmailPoll(ctx, time.Hour) }()
	cancel()
	if err := awaitEmailResult(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled delay, got %v", err)
	}
}
