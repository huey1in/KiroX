package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"

	"reg_go/internal/browser"
	httputil "reg_go/internal/http"
)

var registrarRequests = []struct {
	name string
	call func(*Registrar, string) error
}{
	{"GET", func(r *Registrar, url string) error {
		_, _, _, err := r.DoGet(url, nil)
		return err
	}},
	{"POST", func(r *Registrar, url string) error {
		_, _, err := r.DoPost(url, map[string]string{"value": "test"}, nil)
		return err
	}},
	{"POSTRaw", func(r *Registrar, url string) error {
		_, _, _, err := r.DoPostRaw(url, map[string]string{"value": "test"}, nil)
		return err
	}},
	{"POSTBodyRaw", func(r *Registrar, url string) error {
		_, _, _, err := r.DoPostBodyRaw(url, "value=test", nil)
		return err
	}},
}

func TestRegistrarRequestsCancelInFlight(t *testing.T) {
	for _, request := range registrarRequests {
		for _, flushHeaders := range []bool{false, true} {
			phase := "headers"
			if flushHeaders {
				phase = "body"
			}
			t.Run(request.name+"/"+phase, func(t *testing.T) {
				started := make(chan struct{})
				release := make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					io.Copy(io.Discard, req.Body)
					if flushHeaders {
						io.WriteString(w, "partial")
						w.(http.Flusher).Flush()
					}
					close(started)
					select {
					case <-req.Context().Done():
					case <-release:
					}
				}))
				defer server.Close()
				defer close(release)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				client := httputil.NewTLSClient("", true)
				defer client.CloseIdleConnections()
				r := &Registrar{Ctx: ctx, Client: client}
				done := make(chan error, 1)
				go func() { done <- request.call(r, server.URL) }()
				select {
				case <-started:
				case <-time.After(2 * time.Second):
					t.Fatal("request did not reach the local server")
				}
				cancel()
				select {
				case err := <-done:
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("got %v, want context.Canceled", err)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("request remained blocked after cancellation")
				}
			})
		}
	}
}

type retryErrorClient struct {
	tls_client.HttpClient
	calls atomic.Int32
}

func (c *retryErrorClient) Do(*fhttp.Request) (*fhttp.Response, error) {
	c.calls.Add(1)
	return nil, io.EOF
}

func TestRegistrarRetryBackoffRespectsDeadline(t *testing.T) {
	for _, request := range registrarRequests[:3] {
		t.Run(request.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			client := &retryErrorClient{}
			r := &Registrar{Ctx: ctx, Client: client}
			done := make(chan error, 1)
			go func() { done <- request.call(r, "http://127.0.0.1/") }()
			select {
			case err := <-done:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("got %v, want context.DeadlineExceeded", err)
				}
			case <-time.After(time.Second):
				t.Fatal("retry backoff ignored the deadline")
			}
			if calls := client.calls.Load(); calls != 1 {
				t.Fatalf("got %d requests, want one before cancellation", calls)
			}
		})
	}
}

func TestRegistrarCanceledRequestsDoNotStart(t *testing.T) {
	for _, request := range registrarRequests {
		t.Run(request.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			client := &retryErrorClient{}
			r := &Registrar{Ctx: ctx, Client: client}
			if err := request.call(r, "http://127.0.0.1/"); !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v, want context.Canceled", err)
			}
			if calls := client.calls.Load(); calls != 0 {
				t.Fatalf("canceled task started %d requests", calls)
			}
		})
	}
}

func TestRegistrarNilContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer server.Close()
	client := httputil.NewTLSClient("", true)
	defer client.CloseIdleConnections()
	r := &Registrar{Client: client}
	data, status, _, err := r.DoGet(server.URL, nil)
	if err != nil || status != http.StatusOK || string(data) != "ok" {
		t.Fatalf("got (%q, %d, %v), want (ok, 200, nil)", data, status, err)
	}
	if err := r.wait(0); err != nil {
		t.Fatalf("nil context wait failed: %v", err)
	}
}

func TestBuildProfileHeadersIncludesWAFToken(t *testing.T) {
	r := &Registrar{
		Cfg:      &Config{ProfileBase: "https://profile.example.com"},
		Identity: &browser.BrowserIdentity{},
		Cookies:  map[string]string{"aws-waf-token": "test-waf-token"},
	}
	headers := r.BuildProfileHeaders("https://profile.example.com/")
	if !strings.Contains(headers["Cookie"], "aws-waf-token=test-waf-token") {
		t.Fatalf("Profile Cookie header = %q", headers["Cookie"])
	}
}

func TestBuildHeadersLeavesCookiesToScopedClientJar(t *testing.T) {
	r := &Registrar{
		Identity: &browser.BrowserIdentity{},
		Cookies: map[string]string{
			"aws-usi-authn":         "signin-cookie",
			"aws-user-profile-ubid": "profile-cookie",
		},
	}

	headers := r.BuildHeaders("https://us-east-1.signin.aws/platform/test", "https://us-east-1.signin.aws")
	if _, ok := headers["Cookie"]; ok {
		t.Fatalf("BuildHeaders emitted an unscoped Cookie header: %q", headers["Cookie"])
	}
}
