package email

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func localTCPProxy(t *testing.T, serve func(net.Conn)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var conn net.Conn
	done := make(chan struct{})
	go func() {
		defer close(done)
		accepted, err := listener.Accept()
		if err != nil {
			return
		}
		mu.Lock()
		conn = accepted
		mu.Unlock()
		defer accepted.Close()
		serve(accepted)
	}()
	t.Cleanup(func() {
		listener.Close()
		mu.Lock()
		if conn != nil {
			conn.Close()
		}
		mu.Unlock()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("proxy handler did not stop")
		}
	})
	return listener.Addr().String()
}

func TestHTTPConnectPreservesEarlyTunnelData(t *testing.T) {
	const greeting = "* OK mailbox ready\r\n"
	proxyAddr := localTCPProxy(t, func(conn net.Conn) {
		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			t.Error(err)
			return
		}
		if req.Method != "CONNECT" || req.Host != "mail.example:993" {
			t.Errorf("unexpected tunnel request: %s %s", req.Method, req.Host)
		}
		if req.Header.Get("Proxy-Authorization") != "Basic dXNlcjpwYXNz" {
			t.Error("proxy credentials not forwarded")
		}
		io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"+greeting)
	})
	conn, err := dialThroughProxyContext(context.Background(), "http://user:pass@"+proxyAddr, "tcp", "mail.example:993", 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len(greeting))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != greeting {
		t.Fatalf("tunnel greeting = %q, want %q", got, greeting)
	}
}

func TestProxyHandshakeCancellation(t *testing.T) {
	for _, scheme := range []string{"http", "socks5"} {
		t.Run(scheme, func(t *testing.T) {
			started := make(chan struct{})
			proxyAddr := localTCPProxy(t, func(conn net.Conn) {
				if scheme == "http" {
					if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
						t.Error(err)
					}
				} else {
					var greeting [3]byte
					if _, err := io.ReadFull(conn, greeting[:]); err != nil {
						t.Error(err)
					}
				}
				close(started)
				io.Copy(io.Discard, conn)
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				client := httpClientWithProxy(scheme+"://"+proxyAddr, time.Minute)
				defer client.CloseIdleConnections()
				var conn net.Conn
				var err error
				if scheme == "socks5" {
					conn, err = client.Transport.(*http.Transport).DialContext(ctx, "tcp", "mail.example:993")
				} else {
					conn, err = dialThroughProxyContext(ctx, scheme+"://"+proxyAddr, "tcp", "mail.example:993", time.Minute)
				}
				if conn != nil {
					conn.Close()
				}
				result <- err
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("proxy handshake did not begin")
			}
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("canceled proxy handshake error = %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("canceled handshake waited for the connection timeout")
			}
		})
	}
}

func TestHTTPSProxyVerifiesCertificateBeforeCONNECT(t *testing.T) {
	connectRequests := make(chan struct{}, 1)
	proxy := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		connectRequests <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer proxy.Close()
	conn, err := dialThroughProxyContext(context.Background(), proxy.URL, "tcp", "mail.example:993", 3*time.Second)
	if conn != nil {
		conn.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("untrusted HTTPS proxy error = %v", err)
	}
	select {
	case <-connectRequests:
		t.Fatal("CONNECT sent before verifying the HTTPS proxy certificate")
	default:
	}
}
