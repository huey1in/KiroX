package email

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestOutlookTokenAndMailboxOperationsCancel(t *testing.T) {
	for _, mode := range []string{"imap", "graph"} {
		for _, action := range []string{"token", "baseline", "poll"} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				entered := make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					close(entered)
					<-ctx.Done()
				}))
				defer server.Close()
				previous := outlookTokenAPIBaseURL
				outlookTokenAPIBaseURL = server.URL
				defer func() { outlookTokenAPIBaseURL = previous }()
				result := make(chan error, 1)
				go func() {
					account := OutlookAccount{Mode: mode}
					var err error
					switch action {
					case "baseline":
						_, err = GetOutlookMailboxCountsWithProxy(ctx, account, "")
					case "poll":
						_, err = WaitForOTPWithMailboxCountsProxy(ctx, account, OutlookMailboxCounts{}, 300, 60, "")
					default:
						if mode == "graph" {
							_, err = refreshOutlookGraphToken(ctx, account, "")
						} else {
							_, err = RefreshOutlookTokenWithProxy(ctx, account, "")
						}
					}
					result <- err
				}()
				awaitEmailResult(t, entered)
				cancel()
				if err := awaitEmailResult(t, result); !errors.Is(err, context.Canceled) {
					t.Fatalf("operation did not retain cancellation: %v", err)
				}
			})
		}
	}
}

func TestGraphPollingRequestAndDelayCancel(t *testing.T) {
	for _, mode := range []string{"count", "message", "delay", "body"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "oauth2") {
					io.WriteString(w, `{"access_token":"test"}`)
					return
				}
				if mode == "count" || mode == "body" {
					if mode == "body" {
						w.WriteHeader(http.StatusOK)
						w.(http.Flusher).Flush()
					}
					entered <- struct{}{}
					<-ctx.Done()
					return
				}
				if strings.HasSuffix(r.URL.Path, "/messages") {
					entered <- struct{}{}
					<-ctx.Done()
					return
				}
				if mode == "delay" {
					io.WriteString(w, `{"totalItemCount":0}`)
					entered <- struct{}{}
				} else {
					io.WriteString(w, `{"totalItemCount":1}`)
				}
			}))
			defer server.Close()
			previousToken, previousGraph := outlookTokenAPIBaseURL, outlookGraphAPIBaseURL
			outlookTokenAPIBaseURL, outlookGraphAPIBaseURL = server.URL, server.URL
			defer func() { outlookTokenAPIBaseURL, outlookGraphAPIBaseURL = previousToken, previousGraph }()
			result := make(chan error, 1)
			go func() {
				_, err := waitForOTPGraph(ctx, OutlookAccount{}, OutlookMailboxCounts{Junk: -1}, 300, 60, regexp.MustCompile(`\b(\d{6})\b`), "")
				result <- err
			}()
			awaitEmailResult(t, entered)
			if mode == "delay" {
				time.Sleep(10 * time.Millisecond)
			}
			cancel()
			if err := awaitEmailResult(t, result); !errors.Is(err, context.Canceled) {
				t.Fatalf("Graph cancellation = %v", err)
			}
		})
	}
}

func TestICloudRequestAndPollingDelayCancel(t *testing.T) {
	for _, mode := range []string{"create", "request", "delay"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				entered <- struct{}{}
				if mode != "delay" {
					<-ctx.Done()
				}
			}))
			defer server.Close()
			provider := NewICloudProvider(ctx, ICloudAccount{Email: "test@example.com", MessagesURL: server.URL}, "", "133")
			defer provider.client.CloseIdleConnections()
			result := make(chan error, 1)
			go func() {
				if mode == "create" {
					address := provider.Create()
					if address != "" {
						result <- errors.New("canceled creation returned an address")
						return
					}
					result <- ctx.Err()
					return
				}
				_, err := provider.WaitForCode(300, 60)
				result <- err
			}()
			awaitEmailResult(t, entered)
			if mode == "delay" {
				time.Sleep(10 * time.Millisecond)
			}
			cancel()
			if err := awaitEmailResult(t, result); !errors.Is(err, context.Canceled) {
				t.Fatalf("iCloud cancellation = %v", err)
			}
		})
	}
}

func TestIMAPCanceledContextInterruptsIO(t *testing.T) {
	for _, operation := range []string{"read", "write"} {
		t.Run(operation, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			conn, peer := net.Pipe()
			defer peer.Close()
			client := &imapClient{ctx: ctx, conn: conn, reader: bufio.NewReader(conn)}
			client.stopCancellation = context.AfterFunc(ctx, func() { conn.Close() })
			defer client.close()
			started, result := make(chan struct{}), make(chan error, 1)
			go func() {
				close(started)
				var err error
				if operation == "read" {
					_, err = client.readLine()
				} else {
					_, err = client.sendCommand("NOOP")
				}
				result <- err
			}()
			awaitEmailResult(t, started)
			cancel()
			if err := awaitEmailResult(t, result); !errors.Is(err, context.Canceled) {
				t.Fatalf("IMAP %s cancellation = %v", operation, err)
			}
		})
	}
}
